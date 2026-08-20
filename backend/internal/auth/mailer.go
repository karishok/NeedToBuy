package auth

import (
	"context"
	"fmt"
	"net/smtp"
)

// Mailer sends the OTP code to a parent's email address.
type Mailer interface {
	SendOTP(ctx context.Context, toEmail, code string) error
}

// SMTPMailer sends OTP mail through a plain SMTP relay — mailcatcher in
// dev, a real relay in production.
type SMTPMailer struct {
	Addr string // host:port
	From string // From address on outgoing OTP mail
}

// SendOTP sends a plaintext email containing code to toEmail.
func (m SMTPMailer) SendOTP(_ context.Context, toEmail, code string) error {
	msg := buildOTPMessage(m.From, toEmail, code)
	if err := smtp.SendMail(m.Addr, nil, m.From, []string{toEmail}, msg); err != nil {
		return fmt.Errorf("auth: send otp mail: %w", err)
	}
	return nil
}

// buildOTPMessage renders the raw RFC 5322 message SendOTP hands to SMTP.
func buildOTPMessage(from, to, code string) []byte {
	return []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Код для входа в Нужняшки\r\n\r\nВаш код: %s\nОн действует 10 минут.\n",
		from, to, code))
}
