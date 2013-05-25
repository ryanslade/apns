package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

func createPayload(payload Payload, token string, id uint32) (*rawPayload, error) {
	p := &rawPayload{id: id, created: time.Now()}

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

	// build the actual pdu
	buffer := &bytes.Buffer{}

	// command
	err = binary.Write(buffer, binary.BigEndian, uint8(1))
	if err != nil {
		return nil, err
	}

	// transaction id, optional
	err = binary.Write(buffer, binary.BigEndian, id)
	if err != nil {
		return nil, err
	}

	// expiration time, 1 hour
	err = binary.Write(buffer, binary.BigEndian, uint32(time.Now().Add(1*time.Hour).Unix()))
	if err != nil {
		return nil, err
	}

	// push device token
	err = binary.Write(buffer, binary.BigEndian, uint16(len(btoken)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, btoken)
	if err != nil {
		return nil, err
	}

	// push payload
	err = binary.Write(buffer, binary.BigEndian, uint16(len(bpayload)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, bpayload)
	if err != nil {
		return nil, err
	}

	p.data = buffer.Bytes()

	return p, nil
}
