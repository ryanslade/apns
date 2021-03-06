package apns

import (
	"fmt"
)

// Error returned from Apple's APNS server
type apnsError struct {
	command    uint8
	status     uint8
	identifier uint32
}

var (
	friendlyMessages []string = []string{
		"No Errors",
		"Processing Error",
		"Missing Device Token",
		"Missing Topic",
		"Missing Payload",
		"Invalid Token Size",
		"Invalid Topic Size",
		"Invalid Payload Size",
		"Invalid Token",
	}
)

const (
	unknownErrorMessage = "Unknown error"
)

func (e apnsError) Error() string {
	formatString := fmt.Sprint("APNS Error: %v for payload ", e.identifier)

	if e.command != 8 || e.status < 0 || e.status > uint8(len(friendlyMessages)-1) {
		return fmt.Sprintf(formatString, unknownErrorMessage)
	}

	return fmt.Sprintf(formatString, friendlyMessages[e.status])
}

func (e apnsError) String() string {
	return e.Error()
}
