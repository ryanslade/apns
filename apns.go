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
	payloadsChan chan *rawPayload
	errorChan    chan error
	idChan       chan uint32
	payloads     []*rawPayload
}

const (
	// Close the connection if we don't read anything for this duration
	// after a succesful write. If we don't then Apple silently closes the connection
	// and future writes will fail, but only after some time
	readWindow = time.Minute * 2

	waitBetweenRetries    = time.Minute * 1
	maxWaitBetweenRetries = time.Minute * 30

	apnsServer        = "gateway.push.apple.com:2195"
	apnsServerSandbox = "gateway.sandbox.push.apple.com:2195"

	payloadLifeTime = 5 * time.Minute // How long to hold onto old payloads
)

// Create a new pusher
// certFile and keyFile are paths to the certificate and APNS private key
// Sandbox specifies whether to use the APNS sandbox or production server
func NewPusher(certFile, keyFile string, sandbox bool) (*Pusher, error) {
	newPusher := &Pusher{
		certFile:     certFile,
		keyFile:      keyFile,
		sandbox:      sandbox,
		errorChan:    make(chan error),
		payloadsChan: make(chan *rawPayload),
		idChan:       make(chan uint32),
		payloads:     make([]*rawPayload, 0),
	}

	if err := newPusher.connectAndWait(); err != nil {
		return newPusher, err
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

	return newPusher, nil
}

// Push a message to the designated push token
func (p *Pusher) PushMessage(message, token string) error {
	payload := Payload{Aps: aps{Alert: message}}
	return p.PushPayload(payload, token)
}

// Push a more complex payload to the designated push token
func (p *Pusher) PushPayload(payload Payload, token string) error {
	rawPayload, err := createPayload(payload, token, <-p.idChan)
	if err != nil {
		return errors.New(fmt.Sprintf("Error creating payload: %v", err))
	}
	p.payloadsChan <- rawPayload
	return nil
}

func (pusher *Pusher) connectAndWait() error {
	var conn *tls.Conn

	// Try and connect 
	waitLength := waitBetweenRetries
	for {
		var err error
		conn, err = pusher.connectToAPNS()
		if err == nil {
			break
		}

		if nerr, ok := err.(net.Error); ok {
			// Network error, wait before reconnecting
			log.Printf("Error connecting to APNS: %v, sleeping for %v", nerr, waitLength)
			time.Sleep(waitLength)
			// Keep increasing wait length up to a maximum
			if waitLength < maxWaitBetweenRetries {
				waitLength = waitLength + waitBetweenRetries
			}
			continue
		}

		// Non network error
		return err
	}

	log.Println("Connected...")
	pusher.conn = conn

	// The APNS servers seem to timeout after a few minutes of no activity
	// We'll wait for a set time before the read times out
	// The time will extend after each succesful write 
	pusher.conn.SetReadDeadline(time.Now().Add(readWindow))

	go pusher.handleReads()

	return nil
}

func (pusher *Pusher) handleError(err error) {
	pusher.conn.Close()

	// Only reconnect if we didn't get a network timeout error
	// Since timeouts are most likely caused by us
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		log.Println("Timeout error, not doing auto reconnect")
		return
	}

	// Try and connect forever 
	connectionError := pusher.connectAndWait()
	if connectionError != nil {
		// connectAndWait will only return an error if there was a non network error
		// in this case we have to panic 
		panic("Non network error connecting")
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
			if ae.status > 0 {
				index = index + 1
			}

			toResend := pusher.payloads[index:]
			pusher.payloads = make([]*rawPayload, 0) // Clear sent payloads

			// Resend payloads
			go func() {
				for _, p := range toResend {
					pusher.payloadsChan <- p
				}
			}()
		}
	}

}

func (p *Pusher) cleanupPayloads() {
	livePayloads := make([]*rawPayload, 0)
	for _, v := range p.payloads {
		if time.Since(v.created) > payloadLifeTime {
			continue
		}
		livePayloads = append(livePayloads, v)
	}
	p.payloads = livePayloads
}

func (pusher *Pusher) waitLoop() {
	for {

		select {

		case err := <-pusher.errorChan:
			log.Println("Error:", err)
			pusher.handleError(err)

		case payload := <-pusher.payloadsChan:
			err := pusher.push(payload)

			if err != nil {
				log.Println("Write error:", err)
				go func() {
					log.Println("Resending payload")
					pusher.payloadsChan <- payload
				}()

				pusher.handleError(errors.New("Write error"))
			} else {
				// Only succesfully written payloads are added
				pusher.payloads = append(pusher.payloads, payload)
			}

		}

		pusher.cleanupPayloads()
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

			readErr := binary.Read(buf, binary.BigEndian, &apnsError.command)
			if readErr != nil {
				pusher.errorChan <- readErr
				return
			}

			readErr = binary.Read(buf, binary.BigEndian, &apnsError.status)
			if readErr != nil {
				pusher.errorChan <- readErr
				return
			}

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

func (pusher *Pusher) push(payload *rawPayload) error {
	// write pdu
	log.Println("Writing payload...", payload.id)
	i, err := pusher.conn.Write(payload.data)
	if err != nil {
		return err
	}
	log.Printf("Wrote %v bytes\n", i)

	// No write error, extend timeout window
	// Happy to ignore errors changing the read deadline
	pusher.conn.SetReadDeadline(time.Now().Add(readWindow))

	return nil
}
