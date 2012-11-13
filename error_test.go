package apns

import (
	"testing"
)

func TestErrorMessage(t *testing.T) {
	testCases := []struct {
		actual   apnsError
		expected string
	}{
		{apnsError{8, 0, 1}, "APNS Error: No Errors for payload 1"},
		{apnsError{8, 0, 1}, "APNS Error: No Errors for payload 1"},
		{apnsError{8, 1, 1}, "APNS Error: Processing Error for payload 1"},
		{apnsError{8, 2, 1}, "APNS Error: Missing Device Token for payload 1"},
		{apnsError{8, 3, 1}, "APNS Error: Missing Topic for payload 1"},
		{apnsError{8, 4, 1}, "APNS Error: Missing Payload for payload 1"},
		{apnsError{8, 5, 1}, "APNS Error: Invalid Token Size for payload 1"},
		{apnsError{8, 6, 1}, "APNS Error: Invalid Topic Size for payload 1"},
		{apnsError{8, 7, 1}, "APNS Error: Invalid Payload Size for payload 1"},
		{apnsError{8, 8, 1}, "APNS Error: Invalid Token for payload 1"},
		{apnsError{8, 9, 1}, "APNS Error: Unknown error for payload 1"},
		{apnsError{8, 255, 1}, "APNS Error: Unknown error for payload 1"}}

	for _, v := range testCases {
		actual := v.actual.Error()
		if actual != v.expected {
			t.Errorf("Expected: %v. Got: %v", v.expected, actual)
		}
	}
}
