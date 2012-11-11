package apns

import (
	"testing"
)

func errorMessageHelper(sampleError apnsError, expected string, t *testing.T) {
	actual := sampleError.Error()
	if expected != actual {
		t.Errorf("Expected: %v. Got: %v", expected, actual)
	}
}

func TestErrorMessage(t *testing.T) {
	errorMessageHelper(apnsError{8, 0, 1}, "APNS Error: No Errors for payload 1", t)
	errorMessageHelper(apnsError{8, 1, 1}, "APNS Error: Processing Error for payload 1", t)
	errorMessageHelper(apnsError{8, 2, 1}, "APNS Error: Missing Device Token for payload 1", t)
	errorMessageHelper(apnsError{8, 3, 1}, "APNS Error: Missing Topic for payload 1", t)
	errorMessageHelper(apnsError{8, 4, 1}, "APNS Error: Missing Payload for payload 1", t)
	errorMessageHelper(apnsError{8, 5, 1}, "APNS Error: Invalid Token Size for payload 1", t)
	errorMessageHelper(apnsError{8, 6, 1}, "APNS Error: Invalid Topic Size for payload 1", t)
	errorMessageHelper(apnsError{8, 7, 1}, "APNS Error: Invalid Payload Size for payload 1", t)
	errorMessageHelper(apnsError{8, 8, 1}, "APNS Error: Invalid Token for payload 1", t)

	errorMessageHelper(apnsError{8, 9, 1}, "APNS Error: Unknown error for payload 1", t)
	errorMessageHelper(apnsError{8, 255, 1}, "APNS Error: Unknown error for payload 1", t)
}
