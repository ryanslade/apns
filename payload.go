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

type payload struct {
	data []byte
	id   uint32
}

func createPayload(message, token string, id uint32) (*payload, error) {
	p := &payload{id: id}

	// prepare binary payload from JSON structure
	dictionary := make(map[string]interface{})
	dictionary["aps"] = map[string]string{"alert": message}
	bpayload, err := json.Marshal(dictionary)
	if err != nil {
		return nil, err
	}

	// decode hexadecimal push device token to binary byte array
	btoken, err := hex.DecodeString(token)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error decoding token: %v. Error: %v", token, err))
	}

	// build the actual pdu
	var buffer *bytes.Buffer

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
