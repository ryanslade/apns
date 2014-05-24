package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// The payload details
type Aps struct {
	// The new number to appear in the badge on the app icon.
	// Optional.
	Badge int `json:"badge,omitempty"`
	// The text that appears in the notification.
	Alert string `json:"alert,omitempty"`
	// The sound to play when the alert is received.
	// Optional
	Sound string `json:"sound,omitempty"`
}

// The payload sent to the APNS server
type Payload struct {
	Aps Aps `json:"aps"`
}

type rawPayload struct {
	data    []byte
	id      uint32
	created time.Time
}

type binaryWriter struct {
	w   io.Writer
	err error
}

func (bw *binaryWriter) write(v interface{}) {
	if bw.err != nil {
		return
	}
	bw.err = binary.Write(bw.w, binary.BigEndian, v)
}

func createPayload(payload Payload, token string, id uint32, now time.Time) (*rawPayload, error) {
	p := &rawPayload{id: id, created: now}

	// prepare binary payload from JSON structure
	bpayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// decode hexadecimal push device token to binary byte array
	btoken, err := hex.DecodeString(token)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error decoding token: %v. Error: %v", token, err))
	}

	buffer := new(bytes.Buffer)

	// build the actual pdu
	bw := binaryWriter{w: buffer}

	bw.write(uint8(1))                              // command
	bw.write(id)                                    // transaction id, optional
	bw.write(uint32(now.Add(1 * time.Hour).Unix())) // expiration time, 1 hour
	bw.write(uint16(len(btoken)))                   // push device token
	bw.write(btoken)                                //
	bw.write(uint16(len(bpayload)))                 // push payload
	bw.write(bpayload)                              //

	if bw.err != nil {
		return nil, bw.err
	}

	p.data = buffer.Bytes()

	return p, nil
}
