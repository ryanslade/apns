package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"time"
)

type payload struct {
	data []byte
	id   uint32
}

func createPayload(message, token string, id uint32) (p *payload) {
	// TODO: Error checking
	p = &payload{id: id}

	// prepare binary payload from JSON structure
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
