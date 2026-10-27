package notification

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"
)

type SMTPMailer struct {
	Host, Port, Username, Password, From string
	AllowPlaintext                       bool
}

func SMTPFromEnv() *SMTPMailer {
	return &SMTPMailer{os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"), os.Getenv("SMTP_FROM_ADDRESS"), os.Getenv("NOTIFICATION_SMTP_ALLOW_PLAINTEXT") == "true"}
}
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	failed := func() error { return ErrNotAccepted }
	from, err := mail.ParseAddress(m.From)
	if err != nil || strings.ContainsAny(m.From+msg.To+msg.Subject, "\r\n") {
		return failed()
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return failed()
	}
	if m.Host == "" || m.Port == "" {
		return failed()
	}
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(m.Host, m.Port))
	if err != nil {
		return failed()
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return failed()
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if c.StartTLS(&tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}) != nil {
			return failed()
		}
	} else if !m.AllowPlaintext || m.Username != "" {
		return failed()
	}
	if m.Username != "" {
		if c.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)) != nil {
			return failed()
		}
	}
	if c.Mail(from.Address) != nil {
		return failed()
	}
	if c.Rcpt(to.Address) != nil {
		return failed()
	}
	w, err := c.Data()
	if err != nil {
		return failed()
	}
	message := strings.Join([]string{"From: " + from.String(), "To: " + to.String(), "Subject: " + mime.QEncoding.Encode("UTF-8", msg.Subject), fmt.Sprintf("Message-ID: <auma-notification-%d@%s>", msg.ID, strings.Split(from.Address, "@")[1]), "MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: 8bit", "", strings.ReplaceAll(msg.Body, "\n", "\r\n")}, "\r\n")
	if err := sendDATA(w, []byte(message), func() { _ = conn.Close() }); err != nil {
		return err
	}
	// DATA acceptance establishes success. QUIT failure cannot undo acceptance.
	_ = c.Quit()
	return nil
}

// Write cannot transmit the DATA terminator. On a write failure, abort the
// connection without closing the DATA writer (which would send that terminator).
func sendDATA(w io.WriteCloser, message []byte, abort func()) error {
	n, err := w.Write(message)
	if err != nil || n != len(message) {
		abort()
		return ErrNotAccepted
	}
	// Close sends the terminator and reads the SMTP acceptance response. A lost
	// response is ambiguous; an explicit negative SMTP reply proves rejection.
	if err := w.Close(); err != nil {
		var reply *textproto.Error
		if errors.As(err, &reply) && reply.Code >= 400 {
			return ErrNotAccepted
		}
		return errors.New("delivery outcome unknown")
	}
	return nil
}
