package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSMTP is a minimal plain-text SMTP server on loopback.
type fakeSMTP struct {
	ln       net.Listener
	rcptCode string // reply to RCPT TO, e.g. "250 ok"
	got      chan string
}

func newFakeSMTP(t *testing.T, rcptReply string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, rcptCode: rcptReply, got: make(chan string, 1)}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	w := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	w("220 fake ready")
	var data strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250 fake")
		case strings.HasPrefix(cmd, "MAIL"):
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT"):
			w(f.rcptCode)
		case cmd == "DATA":
			w("354 go ahead")
			for {
				l, _ := r.ReadString('\n')
				if l == ".\r\n" {
					break
				}
				data.WriteString(l)
			}
			f.got <- data.String()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func (f *fakeSMTP) mailer(t *testing.T) *Mailer {
	host, port, _ := net.SplitHostPort(f.ln.Addr().String())
	p, _ := strconv.Atoi(port)
	return NewMailer(host, p, "", "", "Alibi <noreply@alibi.test>")
}

func TestMailerSendsMultipartMessage(t *testing.T) {
	f := newFakeSMTP(t, "250 ok")
	err := f.mailer(t).Send(context.Background(), "player@example.com", "Your code", "code 123456", "<b>123456</b>")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-f.got:
		for _, want := range []string{"To: player@example.com", "Subject: Your code", "multipart/alternative", "code 123456", "<b>123456</b>"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message missing %q:\n%s", want, msg)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the message")
	}
}

func TestMailerPermanentRejectionIsTextprotoError(t *testing.T) {
	f := newFakeSMTP(t, "550 no such mailbox")
	err := f.mailer(t).Send(context.Background(), "ghost@example.com", "s", "t", "h")
	var perr *textproto.Error
	if !errors.As(err, &perr) || perr.Code != 550 {
		t.Fatalf("got %v, want textproto 550", err)
	}
}

func TestMailerRejectsBadRecipients(t *testing.T) {
	m := NewMailer("127.0.0.1", 1, "", "", "noreply@alibi.test")
	for _, to := range []string{"", "not-an-email", "a@b.co\r\nBcc: x@y.z", "Name <a@b.co>"} {
		if err := m.Send(context.Background(), to, "s", "t", "h"); !errors.Is(err, ErrInvalidAddress) {
			t.Errorf("%q: got %v", to, err)
		}
	}
}

func TestMailerConnectFailureIsRetryable(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // nothing listens there now
	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	err := NewMailer(host, p, "", "", "noreply@alibi.test").Send(context.Background(), "a@b.co", "s", "t", "h")
	var perr *textproto.Error
	if err == nil || errors.As(err, &perr) {
		t.Fatalf("got %v, want a non-protocol (retryable) error", err)
	}
}
