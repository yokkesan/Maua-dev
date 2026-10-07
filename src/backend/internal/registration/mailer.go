package registration

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
)

type VerificationMailer interface {
	SendVerificationEmail(
		ctx context.Context,
		email string,
		token string,
	) error
}

type SMTPConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	FromAddress     string
	FromName        string
	VerificationURL string
}

type SMTPMailer struct {
	config SMTPConfig
}

func NewSMTPMailer(config SMTPConfig) *SMTPMailer {
	return &SMTPMailer{
		config: config,
	}
}

func (m *SMTPMailer) SendVerificationEmail(
	ctx context.Context,
	email string,
	token string,
) error {
	if err := m.validateConfig(); err != nil {
		return err
	}

	verificationURL, err := m.buildVerificationURL(token)
	if err != nil {
		return fmt.Errorf("build verification url: %w", err)
	}

	message := m.buildMessage(email, verificationURL)

	address := net.JoinHostPort(
		m.config.Host,
		strconv.Itoa(m.config.Port),
	)

	auth := smtp.PlainAuth(
		"",
		m.config.Username,
		m.config.Password,
		m.config.Host,
	)

	dialer := &net.Dialer{}

	conn, err := dialer.DialContext(
		ctx,
		"tcp",
		address,
	)
	if err != nil {
		return fmt.Errorf("connect smtp server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(
		conn,
		m.config.Host,
	)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: m.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("start smtp tls: %w", err)
		}
	}

	if m.config.Username != "" {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp authentication failed: %w", err)
		}
	}

	if err := client.Mail(m.config.FromAddress); err != nil {
		return fmt.Errorf("set smtp sender: %w", err)
	}

	if err := client.Rcpt(email); err != nil {
		return fmt.Errorf("set smtp recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open smtp message writer: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message writer: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit smtp client: %w", err)
	}

	return nil
}

func (m *SMTPMailer) validateConfig() error {
	if strings.TrimSpace(m.config.Host) == "" {
		return fmt.Errorf("smtp host is required")
	}

	if m.config.Port <= 0 {
		return fmt.Errorf("smtp port is required")
	}

	if _, err := mail.ParseAddress(m.config.FromAddress); err != nil {
		return fmt.Errorf("invalid smtp from address: %w", err)
	}

	if strings.TrimSpace(m.config.VerificationURL) == "" {
		return fmt.Errorf("verification url is required")
	}

	return nil
}

func (m *SMTPMailer) buildVerificationURL(
	token string,
) (string, error) {
	verificationURL, err := url.Parse(m.config.VerificationURL)
	if err != nil {
		return "", err
	}

	query := verificationURL.Query()
	query.Set("token", token)
	verificationURL.RawQuery = query.Encode()

	return verificationURL.String(), nil
}

func (m *SMTPMailer) buildMessage(
	email string,
	verificationURL string,
) []byte {
	from := m.config.FromAddress

	if strings.TrimSpace(m.config.FromName) != "" {
		from = (&mail.Address{
			Name:    m.config.FromName,
			Address: m.config.FromAddress,
		}).String()
	}

	subject := "AUMA 新規登録メールアドレス認証"

	body := fmt.Sprintf(
		`AUMAへの新規登録ありがとうございます。

以下のURLからメールアドレスの認証を行ってください。

%s

このURLには有効期限があります。
このメールに心当たりがない場合は、このメールを破棄してください。
`,
		verificationURL,
	)

	message := strings.Join(
		[]string{
			"From: " + from,
			"To: " + email,
			"Subject: " + subject,
			"MIME-Version: 1.0",
			"Content-Type: text/plain; charset=UTF-8",
			"",
			body,
		},
		"\r\n",
	)

	return []byte(message)
}