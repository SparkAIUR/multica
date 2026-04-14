package service

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"

	"github.com/resend/resend-go/v2"
)

type EmailService struct {
	resendClient *resend.Client
	fromEmail    string
	smtpHost     string
	smtpAddr     string
	smtpUsername string
	smtpPassword string
}

func NewEmailService() *EmailService {
	apiKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	from := strings.TrimSpace(os.Getenv("RESEND_FROM_EMAIL"))
	if from == "" {
		from = "noreply@multica.ai"
	}

	var resendClient *resend.Client
	if apiKey != "" {
		resendClient = resend.NewClient(apiKey)
	}

	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpPort := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if smtpPort == "" {
		smtpPort = "587"
	}
	smtpAddr := ""
	if smtpHost != "" {
		smtpAddr = net.JoinHostPort(smtpHost, smtpPort)
	}
	smtpUsername := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	if smtpFrom != "" {
		from = smtpFrom
	}

	return &EmailService{
		resendClient: resendClient,
		fromEmail:    from,
		smtpHost:     smtpHost,
		smtpAddr:     smtpAddr,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
	}
}

func (s *EmailService) SendVerificationCode(to, code string) error {
	subject := "Your Multica verification code"
	textBody := fmt.Sprintf(
		"Your verification code: %s\n\nThis code expires in 10 minutes.\nIf you didn't request this code, you can safely ignore this email.\n",
		code,
	)

	if s.smtpAddr != "" {
		return s.sendViaSMTP(to, subject, textBody)
	}

	if s.resendClient == nil {
		fmt.Printf("[DEV] Verification code for %s: %s\n", to, code)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		Html: fmt.Sprintf(
			`<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
				<h2>Your verification code</h2>
				<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">%s</p>
				<p>This code expires in 10 minutes.</p>
				<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
			</div>`, code),
	}

	_, err := s.resendClient.Emails.Send(params)
	return err
}

func (s *EmailService) sendViaSMTP(to, subject, body string) error {
	headers := []string{
		fmt.Sprintf("From: %s", s.fromEmail),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	var auth smtp.Auth
	if s.smtpUsername != "" || s.smtpPassword != "" {
		auth = smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)
	}

	return smtp.SendMail(s.smtpAddr, auth, s.fromEmail, []string{to}, []byte(message))
}
