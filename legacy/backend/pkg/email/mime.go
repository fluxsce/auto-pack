package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

func formatFrom(cfg Config) (string, error) {
	cfg.Normalize()
	if cfg.FromName != "" {
		return (&mail.Address{Name: cfg.FromName, Address: cfg.From}).String(), nil
	}
	return cfg.From, nil
}

func buildMIME(cfg Config, msg Message) ([]byte, error) {
	msg.normalize()
	from, err := formatFrom(cfg)
	if err != nil {
		return nil, err
	}

	body, extraHeaders, err := encodeBody(msg)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	if len(msg.Cc) > 0 {
		buf.WriteString("Cc: " + strings.Join(msg.Cc, ", ") + "\r\n")
	}
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	if extraHeaders != "" {
		buf.WriteString(extraHeaders)
		if !strings.HasSuffix(extraHeaders, "\r\n") {
			buf.WriteString("\r\n")
		}
	}
	buf.WriteString("\r\n")
	buf.Write(body)
	return buf.Bytes(), nil
}

// encodeBody returns the message body and optional extra MIME headers (Content-Type block).
func encodeBody(msg Message) ([]byte, string, error) {
	hasText := msg.Text != ""
	hasHTML := msg.HTML != ""
	hasAttach := len(msg.Attachments) > 0

	switch {
	case !hasAttach && hasText && !hasHTML:
		return encodeQuotedPart(msg.Text)
	case !hasAttach && !hasText && hasHTML:
		return encodeHTMLPart(msg.HTML)
	case !hasAttach && hasText && hasHTML:
		return encodeAlternative(msg.Text, msg.HTML)
	case hasAttach:
		return encodeMixed(msg)
	default:
		return nil, "", fmt.Errorf("empty message body")
	}
}

func encodeQuotedPart(body string) ([]byte, string, error) {
	var encoded bytes.Buffer
	w := quotedprintable.NewWriter(&encoded)
	if _, err := w.Write([]byte(body)); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	headers := "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n"
	return encoded.Bytes(), headers, nil
}

func encodeAlternative(text, html string) ([]byte, string, error) {
	boundary := fmt.Sprintf("alt-%d", time.Now().UnixNano())
	var buf bytes.Buffer
	writePart := func(contentType, partBody string) error {
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Type: " + contentType + "\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		w := quotedprintable.NewWriter(&buf)
		if _, err := w.Write([]byte(partBody)); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		// Multipart boundaries must start at the beginning of a line.
		buf.WriteString("\r\n")
		return nil
	}
	if err := writePart("text/plain; charset=utf-8", text); err != nil {
		return nil, "", err
	}
	if err := writePart("text/html; charset=utf-8", html); err != nil {
		return nil, "", err
	}
	buf.WriteString("--" + boundary + "--\r\n")
	headers := fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	return buf.Bytes(), headers, nil
}

func encodeMixed(msg Message) ([]byte, string, error) {
	boundary := fmt.Sprintf("mix-%d", time.Now().UnixNano())
	var buf bytes.Buffer

	if msg.Text != "" || msg.HTML != "" {
		var part []byte
		var partHeaders string
		var err error
		if msg.Text != "" && msg.HTML != "" {
			part, partHeaders, err = encodeAlternative(msg.Text, msg.HTML)
		} else if msg.Text != "" {
			part, partHeaders, err = encodeQuotedPart(msg.Text)
		} else {
			part, partHeaders, err = encodeHTMLPart(msg.HTML)
		}
		if err != nil {
			return nil, "", err
		}
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString(partHeaders)
		buf.WriteString("\r\n")
		buf.Write(part)
		buf.WriteString("\r\n")
	}

	for _, a := range msg.Attachments {
		ct := attachmentContentType(a)
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Type: " + ct + "\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%q\r\n\r\n", a.Filename))
		enc := base64.NewEncoder(base64.StdEncoding, &buf)
		if _, err := enc.Write(a.Data); err != nil {
			return nil, "", err
		}
		if err := enc.Close(); err != nil {
			return nil, "", err
		}
		buf.WriteString("\r\n")
	}

	buf.WriteString("--" + boundary + "--\r\n")
	headers := fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	return buf.Bytes(), headers, nil
}

func encodeHTMLPart(body string) ([]byte, string, error) {
	var encoded bytes.Buffer
	w := quotedprintable.NewWriter(&encoded)
	if _, err := w.Write([]byte(body)); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	headers := "Content-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n"
	return encoded.Bytes(), headers, nil
}

func attachmentContentType(a Attachment) string {
	if ct := strings.TrimSpace(a.ContentType); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
