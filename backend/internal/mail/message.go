package mail

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
)

// buildPlainMessage 构造固定单一 text/plain 报文，继续供 SMTP 测试邮件使用。
func buildPlainMessage(from, to, subject, body string) ([]byte, error) {
	var b bytes.Buffer
	writeMessageHeaders(&b, from, to, subject, "text/plain; charset=utf-8")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQuotedPrintable(&b, body); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// buildMultipartAlternativeMessage 构造业务邮件的 multipart/alternative 报文：
// 先 text/plain 后 text/html，两部分均 charset=utf-8 + quoted-printable，结束边界完整。
func buildMultipartAlternativeMessage(from, to, subject, textBody, htmlBody string) ([]byte, error) {
	var parts bytes.Buffer
	mw := multipart.NewWriter(&parts)
	boundary := mw.Boundary()
	if err := writeMultipartPart(mw, "text/plain; charset=utf-8", textBody); err != nil {
		return nil, err
	}
	if err := writeMultipartPart(mw, "text/html; charset=utf-8", htmlBody); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart 报文失败: %w", err)
	}

	var b bytes.Buffer
	writeMessageHeaders(&b, from, to, subject, `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	b.Write(parts.Bytes())
	return b.Bytes(), nil
}

func writeMessageHeaders(b *bytes.Buffer, from, to, subject, contentType string) {
	b.WriteString("From: ")
	b.WriteString(sanitizeHeader(from))
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(sanitizeHeader(to))
	b.WriteString("\r\n")
	b.WriteString("Subject: ")
	b.WriteString(encodeSubject(subject))
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: ")
	b.WriteString(contentType)
	b.WriteString("\r\n")
}

func encodeSubject(subject string) string {
	return mime.QEncoding.Encode("utf-8", sanitizeHeader(subject))
}

func writeMultipartPart(mw *multipart.Writer, contentType, body string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := mw.CreatePart(header)
	if err != nil {
		return fmt.Errorf("创建邮件正文部分失败: %w", err)
	}
	if err := writeQuotedPrintable(part, body); err != nil {
		return err
	}
	return nil
}

func writeQuotedPrintable(w interface{ Write([]byte) (int, error) }, body string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(normalizeCRLF(body))); err != nil {
		_ = qp.Close()
		return fmt.Errorf("写入 quoted-printable 正文失败: %w", err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("关闭 quoted-printable 正文失败: %w", err)
	}
	return nil
}

// normalizeCRLF 统一报文正文换行为 CRLF；输入中的 CRLF/CR 先归一为 LF，再统一升级。
func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
