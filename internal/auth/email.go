package auth

import (
	"fmt"
	"net/smtp"
)

// EmailSender defines the interface for sending transactional emails
type EmailSender interface {
	SendPasswordReset(email, resetURL string) error
}

// SMTPConfig holds configuration for SMTP email delivery
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPEmailSender sends emails via SMTP
type SMTPEmailSender struct {
	cfg SMTPConfig
}

// NewSMTPEmailSender creates a new SMTP email sender
func NewSMTPEmailSender(cfg SMTPConfig) *SMTPEmailSender {
	return &SMTPEmailSender{cfg: cfg}
}

// SendPasswordReset sends a password reset email with the given reset URL
func (s *SMTPEmailSender) SendPasswordReset(email, resetURL string) error {
	subject := "Subject: Password Reset Request\r\n"
	to := fmt.Sprintf("To: %s\r\n", email)
	mime := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"
	body := fmt.Sprintf("You requested a password reset. Click the link below to reset your password:\r\n\r\n%s\r\n\r\nIf you did not request this, please ignore this email.\r\n", resetURL)

	msg := []byte(subject + to + mime + body)
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" && s.cfg.Password != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{email}, msg)
}

// Ensure SMTPEmailSender implements EmailSender
var _ EmailSender = (*SMTPEmailSender)(nil)

// NoopEmailSender logs emails instead of sending them (useful for development)
type NoopEmailSender struct {
	LogFunc func(email, resetURL string)
}

// NewNoopEmailSender creates a new no-op email sender
func NewNoopEmailSender() *NoopEmailSender {
	return &NoopEmailSender{
		LogFunc: func(email, resetURL string) {
			fmt.Printf("[NOOP EMAIL] To: %s | Reset URL: %s\n", email, resetURL)
		},
	}
}

// SendPasswordReset logs the password reset email instead of sending it
func (s *NoopEmailSender) SendPasswordReset(email, resetURL string) error {
	if s.LogFunc != nil {
		s.LogFunc(email, resetURL)
	}
	return nil
}

// Ensure NoopEmailSender implements EmailSender
var _ EmailSender = (*NoopEmailSender)(nil)
