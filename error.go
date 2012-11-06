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

func (e apnsError) Error() string {
	return fmt.Sprintf("APNS Error: Command %v, Status %v, Identifier %v", e.command, e.status, e.identifier)
}

func (e apnsError) String() string {
	return e.Error()
}
