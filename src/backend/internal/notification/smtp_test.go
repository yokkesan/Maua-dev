package notification

import (
	"errors"
	"io"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A pipe-only SMTP peer captures test data; no socket or external Mailer is used.
type failingDataConnection struct {
	net.Conn
	fail atomic.Bool
}

func (c *failingDataConnection) Write(data []byte) (int, error) {
	if c.fail.Load() {
		return 0, io.ErrClosedPipe
	}
	return c.Conn.Write(data)
}
func smtpDATAFixture(t *testing.T, dropAcceptance bool) (io.WriteCloser, *failingDataConnection, <-chan bool) {
	t.Helper()
	client, server := net.Pipe()
	conn := &failingDataConnection{Conn: client}
	done := make(chan bool, 1)
	go func() {
		defer server.Close()
		peer := textproto.NewConn(server)
		if peer.PrintfLine("220 test SMTP") != nil {
			done <- false
			return
		}
		for {
			line, err := peer.ReadLine()
			if err != nil {
				done <- false
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				err = peer.PrintfLine("250 test SMTP")
			case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
				err = peer.PrintfLine("250 OK")
			case line == "DATA":
				if peer.PrintfLine("354 Continue") != nil {
					done <- false
					return
				}
				_, err := peer.ReadDotBytes()
				done <- err == nil
				if err == nil && !dropAcceptance {
					_ = peer.PrintfLine("250 accepted")
				}
				return
			default:
				err = peer.PrintfLine("500 unexpected command")
			}
			if err != nil {
				done <- false
				return
			}
		}
	}()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	c, err := smtp.NewClient(conn, "test.invalid")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err = c.Mail("sender@example.com"); err != nil {
		t.Fatal(err)
	}
	if err = c.Rcpt("recipient@example.com"); err != nil {
		t.Fatal(err)
	}
	w, err := c.Data()
	if err != nil {
		t.Fatal(err)
	}
	return w, conn, done
}
func TestSMTPWriteFailureIsNotAccepted(t *testing.T) {
	w, conn, done := smtpDATAFixture(t, false)
	conn.fail.Store(true)
	aborted := false
	err := sendDATA(w, []byte(strings.Repeat("test body\n", 2000)), func() { aborted = true; _ = conn.Close() })
	if !errors.Is(err, ErrNotAccepted) || !aborted {
		t.Fatal("DATA write failure must abort and be retryable")
	}
	if <-done {
		t.Fatal("DATA terminator was sent after a write failure")
	}
}
func TestSMTPAcceptanceResponseLossIsUnknown(t *testing.T) {
	w, conn, done := smtpDATAFixture(t, true)
	err := sendDATA(w, []byte("test body\r\n"), func() { _ = conn.Close() })
	if err == nil || errors.Is(err, ErrNotAccepted) {
		t.Fatal("lost acceptance response must remain unknown")
	}
	if !<-done {
		t.Fatal("fixture did not receive the complete DATA body")
	}
}
func TestSMTPAcceptanceSuccess(t *testing.T) {
	w, conn, done := smtpDATAFixture(t, false)
	if err := sendDATA(w, []byte("test body\r\n"), func() { _ = conn.Close() }); err != nil {
		t.Fatal(err)
	}
	if !<-done {
		t.Fatal("complete DATA missing")
	}
}
