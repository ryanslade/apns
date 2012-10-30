package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"time"
)

type payload struct {
	data []byte
	id   uint32
}

type FeedbackResponse struct {
	timestamp time.Time
	token     string
}

type Pusher struct {
	certFile     string
	keyFile      string
	sandbox      bool
	conn         *tls.Conn
	payloadsChan chan *payload
	errorChan    chan *apnsError
	idChan       chan uint32
	payloads     []*payload
}

const (
	readWindow             = time.Minute * 2
	connectionRetries      = 3
	waitBetweenConnections = time.Minute * 1

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
		errorChan:    make(chan *apnsError),
		payloadsChan: make(chan *payload, 1024),
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
	go newPusher.listen()

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
	p.payloadsChan <- payload
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

		time.Sleep(waitBetweenConnections)
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

func (pusher *Pusher) handleError(err *apnsError) {
	pusher.conn.Close()

	// Only reconnect if we didn't get a network timeout error
	// Since timeouts are most likely caused by us
	if nerr, ok := err.otherError.(net.Error); ok && nerr.Timeout() {
		log.Println("Timeout error, not doing auto reconnect")
	} else {
		// Try and connect, retries a few times if there is an issue
		connectionError := pusher.connectAndWait()
		// TODO, would cause anything using the package to fail
		if connectionError != nil {
			panic("Error connecting to APNS")
		}
	}

	if err.identifier > 0 {
		log.Printf("Error with payload: %v\n", err.identifier)

		// Throw away all items up to and including the failed payload 
		// TODO: Optimise
		index := 0
		for i, v := range pusher.payloads {
			if v.id == err.identifier {
				index = i
				break
			}
		}

		// Throw away the failed payload too if it had been rejected by apple
		if err.command > 0 {
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

func (pusher *Pusher) listen() {
	log.Println("Waiting for messages to push...")
	for {
		select {

		case err := <-pusher.errorChan:
			log.Printf("Error: %v", err)
			pusher.handleError(err)

		case payload := <-pusher.payloadsChan:
			log.Println("Received payload on channel.")
			pusher.payloads = append(pusher.payloads, payload)
			err := pusher.push(payload)
			if err != nil {
				log.Printf("Write error: %v\n", err)
				pusher.connectAndWait()
				log.Println("Resending payload")
				// TODO: What do we do if it fails again?
				pusher.push(payload)
			}

		}

	}
}

func (pusher *Pusher) handleReads() {
	apnsError := new(apnsError)

	readb := make([]byte, 6)
	log.Println("Waiting to read response...")
	n, err := pusher.conn.Read(readb)
	if n == 6 {
		buf := bytes.NewBuffer(readb)

		binary.Read(buf, binary.BigEndian, &apnsError.command)
		binary.Read(buf, binary.BigEndian, &apnsError.status)
		binary.Read(buf, binary.BigEndian, &apnsError.identifier)
	} else if err != nil {
		apnsError.otherError = err
	}

	pusher.errorChan <- apnsError
}

func createPayload(message, token string, id uint32) (p *payload) {
	p = &payload{id: id}

	// prepare inary payload from JSON structure
	dictionary := make(map[string]interface{})
	dictionary["aps"] = map[string]string{"alert": message}
	bpayload, _ := json.Marshal(dictionary)

	// decode hexadecimal push device token to binary byte array
	btoken, _ := hex.DecodeString(token)

	// build the actual pdu
	buffer := new(bytes.Buffer)

	// command
	binary.Write(buffer, binary.BigEndian, uint8(1))

	// transaction id, optional
	binary.Write(buffer, binary.BigEndian, id)

	// expiration time, 1 hour
	binary.Write(buffer, binary.BigEndian, uint32(time.Now().Add(1*time.Hour).Unix()))

	// push device token
	binary.Write(buffer, binary.BigEndian, uint16(len(btoken)))
	binary.Write(buffer, binary.BigEndian, btoken)

	// push payload
	binary.Write(buffer, binary.BigEndian, uint16(len(bpayload)))
	binary.Write(buffer, binary.BigEndian, bpayload)

	p.data = buffer.Bytes()

	return
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
		log.Fatalf("Certificate or key error: %v\n", err)
		return
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// connect to the APNS server 
	log.Println("Dialing server...")

	tlsConn, err = tls.Dial("tcp", server, conf)
	if err != nil {
		log.Printf("connection error: %s\n", err)
		return
	}

	return
}

func (pusher *Pusher) push(payload *payload) (err error) {
	// write pdu
	log.Printf("Writing payload... %v\n", payload.id)
	i, err := pusher.conn.Write(payload.data)
	if err != nil {
		return
	}
	log.Printf("Wrote %v bytes\n", i)

	// No write error, extend timeout window
	pusher.conn.SetReadDeadline(time.Now().Add(readWindow))

	return
}
