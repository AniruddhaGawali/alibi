package email

import (
	"fmt"
	"html"
	"time"
)

// expiryPhrase says how long the code stays valid, derived from m.ExpiresAt
// (minutes left, rounded up) so it follows OTP_TTL_MINUTES. Without an expiry it
// stays vague rather than guessing a number.
func expiryPhrase(m Message, now time.Time) string {
	if m.ExpiresAt <= 0 {
		return "expires soon"
	}
	left := time.Unix(m.ExpiresAt, 0).Sub(now)
	mins := int((left + time.Minute - 1) / time.Minute)
	switch {
	case mins <= 1:
		return "expires in 1 minute"
	default:
		return fmt.Sprintf("expires in %d minutes", mins)
	}
}

// render builds the subject and the plain-text and HTML bodies for m.
func render(m Message, now time.Time) (subject, text, htmlBody string, err error) {
	switch m.Type {
	case TypeSignup:
		expires := expiryPhrase(m, now)
		subject = "Your Alibi verification code"
		text = fmt.Sprintf("Welcome to Alibi, detective.\n\nYour verification code is: %s\n\n"+
			"It %s. If you did not sign up, you can ignore this email.\n", m.OTP, expires)
		htmlBody = fmt.Sprintf(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#1b1410;font-family:Georgia,serif;color:#f3e3c3">
<table role="presentation" width="100%%" style="max-width:480px;margin:0 auto;background:#2a1f17;border:1px solid #c98a2b">
<tr><td style="padding:28px">
<h1 style="margin:0 0 12px;font-size:22px;color:#f0b44c">Welcome to Alibi, detective</h1>
<p style="margin:0 0 20px">Use this code to finish creating your account:</p>
<p style="margin:0 0 20px;font-size:34px;letter-spacing:8px;font-weight:bold;color:#f0b44c">%s</p>
<p style="margin:0;font-size:13px;color:#b9a47f">It %s. If you did not sign up, you can ignore this email.</p>
</td></tr></table></body></html>`, html.EscapeString(m.OTP), expires)
		return subject, text, htmlBody, nil
	}
	return "", "", "", fmt.Errorf("email: unknown message type %q", m.Type)
}
