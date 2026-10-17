package utils

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Mailer sends email through an SMTP relay (Brevo) using net/smtp.
//
// Port 465 uses implicit TLS; any other port must offer STARTTLS, and
// credentials are never sent over an unencrypted connection (loopback hosts
// excepted, for local test servers).
type Mailer struct {
	host, user, pass string
	port             int
	from             string
	timeout          time.Duration
}

func NewMailer(host string, port int, user, pass, from string) *Mailer {
	return &Mailer{host: host, port: port, user: user, pass: pass, from: from, timeout: 20 * time.Second}
}

// ErrInvalidAddress is returned for a recipient that is not a plain address.
var ErrInvalidAddress = errors.New("email: invalid address")

// Send delivers one message with a plain-text and an HTML alternative. ctx
// bounds the whole SMTP conversation. An error of type *textproto.Error with
// Code >= 500 is a permanent rejection (e.g. unknown mailbox); other errors
// (network, 4xx) are worth retrying.
func (m *Mailer) Send(ctx context.Context, to, subject, text, html string) error {
	addr, err := mail.ParseAddress(to)
	if err != nil || addr.Address != to || strings.ContainsAny(to, "\r\n") {
		return ErrInvalidAddress
	}
	fromAddr, err := mail.ParseAddress(m.from)
	if err != nil {
		return fmt.Errorf("email: invalid sender %q", m.from)
	}
	msg, err := buildMessage(fromAddr.Address, to, subject, text, html)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	conn, err := m.dial(ctx)
	if err != nil {
		return fmt.Errorf("email: connect: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	// Closing the connection when ctx ends unblocks any pending read or write.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("email: smtp handshake: %w", err)
	}
	defer c.Close()

	if _, isTLS := conn.(*tls.Conn); !isTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("email: starttls: %w", err)
			}
		} else if !isLoopback(m.host) {
			return errors.New("email: server does not offer STARTTLS; refusing to continue unencrypted")
		}
	}
	if m.user != "" {
		if err := c.Auth(smtp.PlainAuth("", m.user, m.pass, m.host)); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}
	if err := c.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("email: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("email: RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	_ = c.Quit()
	return nil
}

func (m *Mailer) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	d := &net.Dialer{}
	if m.port == 465 {
		return (&tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", addr)
	}
	return d.DialContext(ctx, "tcp", addr)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// buildMessage renders a multipart/alternative message. Header values are
// checked for line breaks so callers cannot inject extra headers.
func buildMessage(from, to, subject, text, html string) ([]byte, error) {
	for _, v := range []string{from, to, subject} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("email: line break in header value")
		}
	}
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	boundary := "alibi-" + hex.EncodeToString(b[:])

	var sb strings.Builder
	h := func(k, v string) { sb.WriteString(k + ": " + v + "\r\n") }
	h("From", from)
	h("To", to)
	h("Subject", mime.QEncoding.Encode("utf-8", subject))
	h("Date", time.Now().UTC().Format(time.RFC1123Z))
	h("Message-ID", "<"+hex.EncodeToString(b[:])+"@alibi>")
	h("MIME-Version", "1.0")
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	sb.WriteString("\r\n")
	part := func(ctype, body string) {
		sb.WriteString("--" + boundary + "\r\n")
		sb.WriteString("Content-Type: " + ctype + "; charset=utf-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		sb.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
		sb.WriteString("\r\n")
	}
	part("text/plain", text)
	part("text/html", html)
	sb.WriteString("--" + boundary + "--\r\n")
	return []byte(sb.String()), nil
}
