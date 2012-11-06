package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type Pusher struct {
	certFile     string
	keyFile      string
	sandbox      bool
	conn         *tls.Conn
	payloadsChan chan *payload
	errorChan    chan error
	idChan       chan uint32
	payloads     []*payload
}

const (
	// Close the connection if we don't read anything for this duration
	// after a succesful write. If we don't then Apple silently closes the connection
	// and future writes will fail, but only after some time
	readWindow         = time.Minute * 2
	connectionRetries  = 3
	waitBetweenRetries = time.Minute * 1

	apnsServer        = "gateway.push.apple.com:2195"
	apnsServerSandbox = "gateway.sandbox.push.apple.com:2195"

	feedbackServer        = "feedback.push.apple.com:2196"
	feedbackServerSandbox = "feedback.sandbox.push.apple.com:2196"
)

// Create a new pusher
// certFile and keyFile are paths to the certificate and APNS private key
// Sandbox specifies whether to use the APNS sandbox or production server
func NewPusher(certFile, keyFile string, sandbox bool) (newPusher *Pusher, err error) {
	newPusher = &Pusher{
		certFile:     certFile,
		keyFile:      keyFile,
		sandbox:      sandbox,
		errorChan:    make(chan error),
		payloadsChan: make(chan *payload),
		idChan:       make(chan uint32),
		payloads:     make([]*payload, 0),
	}

	if err = newPusher.connectAndWait(); err != nil {
		return
	}

	// Generate id's
	go func() {
		id := uint32(0)
		for {
			id = id + 1
			newPusher.idChan <- id
		}
	}()

	// listen runs in the background waiting on the payloads channel
	go newPusher.waitLoop()

	return
}

// Shutdown gracefully 
// (not implemened yet)
func (p *Pusher) Shutdown() error {
	// Shut down gracefully
	// TODO
	return nil
}

// Push a message to the designated push token
// This is a non blocking method
func (p *Pusher) Push(message, token string) {
	payload := createPayload(message, token, <-p.idChan)
	go func() { p.payloadsChan <- payload }()
}

func (pusher *Pusher) connectAndWait() (err error) {
	var conn *tls.Conn
	for i := 1; ; i++ {
		conn, err = pusher.connectToAPNS()
		if err == nil {
			break
		}

		// We've tried enough times, give up
		if err != nil && i == connectionRetries {
			return
		}

		time.Sleep(waitBetweenRetries)
	}

	log.Println("Connected...")
	pusher.conn = conn

	// The APNS servers seem to timeout after a few minutes of no activity
	// We'll wait for a set time before the read times out
	// The time will extend after each succesful write 
	pusher.conn.SetReadDeadline(time.Now().Add(readWindow))

	go pusher.handleReads()

	return
}

func (pusher *Pusher) handleError(err error) {
	pusher.conn.Close()

	// Only reconnect if we didn't get a network timeout error
	// Since timeouts are most likely caused by us
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		log.Println("Timeout error, not doing auto reconnect")
	} else {
		// Try and connect, retries a few times if there is an issue
		connectionError := pusher.connectAndWait()
		// TODO, would cause anything using the package to fail
		if connectionError != nil {
			panic("Error connecting to APNS")
		}
	}

	if ae, ok := err.(apnsError); ok {
		if ae.identifier > 0 {
			log.Println("Error with payload:", ae.identifier)

			// Throw away all items up to and including the failed payload 
			// TODO: Optimise
			index := 0
			for i, v := range pusher.payloads {
				if v.id == ae.identifier {
					index = i
					break
				}
			}

			// Throw away the failed payload too if it had been rejected by apple
			if ae.command > 0 {
				index = index + 1
			}

			toResend := pusher.payloads[index:]
			pusher.payloads = make([]*payload, 0) // Clear sent payloads

			// Resend payloads
			go func() {
				for _, p := range toResend {
					pusher.payloadsChan <- p
				}
			}()
		}
	}

}

func (pusher *Pusher) waitLoop() {
	log.Println("Waiting for messages to push...")
	for {
		select {

		case err := <-pusher.errorChan:
			log.Println("Error:", err)
			pusher.handleError(err)

		case payload := <-pusher.payloadsChan:
			log.Println("Received payload on channel.")
			pusher.payloads = append(pusher.payloads, payload)
			err := pusher.push(payload)
			if err != nil {
				log.Println("Write error:", err)
				pusher.connectAndWait()
				log.Println("Resending payload")
				// TODO: What do we do if it fails again?
				pusher.push(payload)
			}

		}

	}
}

func (pusher *Pusher) handleReads() {
	readb := make([]byte, 6)
	log.Println("Waiting to read response...")
	n, err := pusher.conn.Read(readb)

	if err != nil {
		pusher.errorChan <- err
	} else {
		// We expect 6
		if n == 6 {
			var apnsError apnsError

			buf := bytes.NewBuffer(readb)

			var readErr error
			readErr = binary.Read(buf, binary.BigEndian, &apnsError.command)
			readErr = binary.Read(buf, binary.BigEndian, &apnsError.status)
			readErr = binary.Read(buf, binary.BigEndian, &apnsError.identifier)
			if readErr != nil {
				pusher.errorChan <- readErr
				return
			}

			pusher.errorChan <- apnsError // This is the happy path
		} else {
			pusher.errorChan <- errors.New(fmt.Sprintf("Expected 6 bytes from APNS read, got %v", n))
		}
	}
}

func (p *Pusher) connectToAPNS() (tlsConn *tls.Conn, err error) {
	server := apnsServer
	if p.sandbox {
		server = apnsServerSandbox
	}

	return p.connect(server)
}

func (p *Pusher) connectToFeedback() (tlsConn *tls.Conn, err error) {
	server := feedbackServer
	if p.sandbox {
		server = feedbackServerSandbox
	}

	return p.connect(server)
}

func (pusher *Pusher) connect(server string) (tlsConn *tls.Conn, err error) {
	// load certificates and setup config
	log.Println("Loading certificates...")
	cert, err := tls.LoadX509KeyPair(pusher.certFile, pusher.keyFile)
	if err != nil {
		log.Println("Certificate or key error:", err)
		return
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// connect to the APNS server 
	log.Println("Dialing server...")

	tlsConn, err = tls.Dial("tcp", server, conf)
	if err != nil {
		log.Println("Connection error:", err)
		return
	}

	return
}

func (pusher *Pusher) push(payload *payload) (err error) {
	// write pdu
	log.Println("Writing payload...", payload.id)
	i, err := pusher.conn.Write(payload.data)
	if err != nil {
		return
	}
	log.Printf("Wrote %v bytes\n", i)

	// No write error, extend timeout window
	pusher.conn.SetReadDeadline(time.Now().Add(readWindow))

	return
}
