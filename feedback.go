package apns

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log"
	"time"
)

const (
	feedbackServer        = "feedback.push.apple.com:2196"
	feedbackServerSandbox = "feedback.sandbox.push.apple.com:2196"
)

// An item returned from the APNS Feedback service
type Feedback struct {
	// The time the feedback item was created by Apple (UTC)
	TimeStamp time.Time
	// The device token that is no longer valid
	Token string
}

func (p *Pusher) connectToFeedback() (tlsConn *tls.Conn, err error) {
	server := feedbackServer
	if p.sandbox {
		server = feedbackServerSandbox
	}

	return p.connect(server)
}

// Connect to the APNS Feedback service and return relevent tokens.
// Details of the Feedback service are available here:
// http://developer.apple.com/library/mac/#documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/CommunicatingWIthAPS.html#//apple_ref/doc/uid/TP40008194-CH101-SW3
func (p *Pusher) GetFeedback() ([]Feedback, error) {
	feedback := make([]Feedback, 0)

	conn, err := p.connectToFeedback()
	if err != nil {
		return feedback, err
	}

	defer conn.Close()

	var seconds uint32
	var tokenLength int16

	for {
		err = binary.Read(conn, binary.BigEndian, &seconds)
		if err == io.EOF {
			// EOF here is not an error, it just means there are no more feedback items
			log.Println("Feedback service finished")
			return feedback, nil
		}

		if err != nil {
			log.Println("Error decoding timestamp:", err)
			return feedback, err
		}

		timestamp := time.Unix(int64(seconds), 0)

		err = binary.Read(conn, binary.BigEndian, &tokenLength)
		if err != nil {
			log.Println("Error decoding token length:", err)
			return feedback, err
		}

		token := make([]byte, tokenLength)
		err = binary.Read(conn, binary.BigEndian, &token)
		if err != nil {
			log.Println("Error decoding token:", err)
			return feedback, err
		}
		tokenString := hex.EncodeToString(token)

		log.Printf("Timestamp: %v, Token: %v", timestamp, tokenString)

		feedback = append(feedback, Feedback{timestamp, tokenString})
	}
}
