package auth

import (
	"strings"
	"testing"
)

func TestBuildOTPMessage_ContainsCodeAndRecipient(t *testing.T) {
	msg := string(buildOTPMessage("no-reply@needtobuy.local", "parent@example.com", "123456"))

	if !strings.Contains(msg, "123456") {
		t.Errorf("message does not contain the code: %s", msg)
	}
	if !strings.Contains(msg, "To: parent@example.com") {
		t.Errorf("message does not address the recipient: %s", msg)
	}
	if !strings.Contains(msg, "From: no-reply@needtobuy.local") {
		t.Errorf("message does not set the sender: %s", msg)
	}
}
