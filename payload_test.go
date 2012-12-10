package apns

import (
	"testing"
)

func TestCreatePayloadDoesNotPanic(t *testing.T) {
	rawPayload := Payload{Aps: aps{Alert: "Message"}}
	payload, err := createPayload(rawPayload, "1234567812345678123456781234567812345678123456781234567812345678", 1)

	if err != nil {
		t.Error("Error should not be nil, got:", err)
	}

	if payload == nil {
		t.Error("Payload should not be nil")
	}
}
