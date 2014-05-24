package apns

import (
	"reflect"
	"testing"
	"time"
)

var knownTime = time.Date(2014, 01, 01, 01, 01, 01, 01, time.UTC)

func TestCreatePayloadDoesNotPanic(t *testing.T) {
	rawPayload := Payload{Aps: Aps{Alert: "Message"}}
	payload, err := createPayload(rawPayload, "1234567812345678123456781234567812345678123456781234567812345678", 1, knownTime)

	if err != nil {
		t.Error("Error should not be nil, got:", err)
	}

	if payload == nil {
		t.Error("Payload should not be nil")
	}
}

func TestCreatePayloadData(t *testing.T) {
	rawPayload := Payload{Aps: Aps{Alert: "Message"}}
	payload, err := createPayload(rawPayload, "1234567812345678123456781234567812345678123456781234567812345678", 1, knownTime)
	if err != nil {
		t.Error("Error should not be nil, got:", err)
	}
	expectedData := []byte{1, 0, 0, 0, 1, 82, 195, 118, 221, 0, 32, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 18, 52, 86, 120, 0, 27, 123, 34, 97, 112, 115, 34, 58, 123, 34, 97, 108, 101, 114, 116, 34, 58, 34, 77, 101, 115, 115, 97, 103, 101, 34, 125, 125}

	if !reflect.DeepEqual(expectedData, payload.data) {
		t.Errorf("Payload data incorrect")
		t.Error(payload.data)
	}
}
