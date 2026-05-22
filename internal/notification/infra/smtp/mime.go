package smtp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"strings"
	"time"
)

// BuildMessage produces a MIME envelope suitable for net/smtp.SendMail.
// Rules:
//   - Subject is Q-encoded (UTF-8 safe across all clients).
//   - When htmlBody is empty, the envelope is text/plain only (no
//     multipart wrapper — single-part is friendlier to legacy MTAs).
//   - When htmlBody is non-empty, multipart/alternative is used with text
//     first and html second, per RFC 2046 §5.1.4 (clients pick the last
//     part they can render).
//   - Both bodies have their LF normalised to CRLF for the wire; the
//     renderer always produces LF and we never want to depend on its callers.
//   - A Message-ID is generated for every send so the audit trail can
//     correlate SMTP server logs.
func BuildMessage(from, to, subject, textBody, htmlBody string) ([]byte, string) {
	messageID := generateMessageID(from)

	var sb strings.Builder
	sb.WriteString("From: ")
	sb.WriteString(strings.TrimSpace(from))
	sb.WriteString("\r\n")
	sb.WriteString("To: ")
	sb.WriteString(strings.TrimSpace(to))
	sb.WriteString("\r\n")
	sb.WriteString("Subject: ")
	sb.WriteString(mime.QEncoding.Encode("UTF-8", subject))
	sb.WriteString("\r\n")
	sb.WriteString("Message-ID: ")
	sb.WriteString(messageID)
	sb.WriteString("\r\n")
	sb.WriteString("Date: ")
	sb.WriteString(time.Now().UTC().Format(time.RFC1123Z))
	sb.WriteString("\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")

	htmlBody = strings.TrimSpace(htmlBody)
	if htmlBody == "" {
		sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(crlfBody(textBody))
		return []byte(sb.String()), messageID
	}

	boundary := generateBoundary()
	sb.WriteString("Content-Type: multipart/alternative; boundary=\"")
	sb.WriteString(boundary)
	sb.WriteString("\"\r\n\r\n")

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	sb.WriteString(crlfBody(textBody))
	sb.WriteString("\r\n")

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	sb.WriteString(crlfBody(htmlBody))
	sb.WriteString("\r\n")

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("--\r\n")

	return []byte(sb.String()), messageID
}

// crlfBody normalises any LF-only body to CRLF and trims the trailing
// CRLF/LF/CR so the boundary that follows starts on a clean line.
func crlfBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	body = strings.TrimRight(body, "\r\n")
	return body
}

func generateMessageID(from string) string {
	domain := "cobo.local"
	if at := strings.LastIndex(from, "@"); at >= 0 && at < len(from)-1 {
		// Strip any trailing > on a "Name <addr@host>" form.
		domain = strings.TrimRight(from[at+1:], ">")
		domain = strings.TrimSpace(domain)
		if domain == "" {
			domain = "cobo.local"
		}
	}
	var buf [12]byte
	_, _ = rand.Read(buf[:])
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UTC().UnixNano(), hex.EncodeToString(buf[:]), domain)
}

func generateBoundary() string {
	var buf [12]byte
	_, _ = rand.Read(buf[:])
	return "COBO_BOUNDARY_" + hex.EncodeToString(buf[:])
}
