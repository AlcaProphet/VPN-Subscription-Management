package mail

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func decodeSubject(t *testing.T, header mail.Header) string {
	t.Helper()
	dec := new(mime.WordDecoder)
	raw := header.Get("Subject")
	got, err := dec.DecodeHeader(raw)
	if err != nil {
		t.Fatalf("解码主题失败: %v", err)
	}
	return got
}

func parsePartMediaType(t *testing.T, ct string) string {
	t.Helper()
	media, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("解析 part Content-Type 失败: %v", err)
	}
	if params["charset"] != "utf-8" {
		t.Fatalf("part charset 应为 utf-8: %q", ct)
	}
	return media
}

type capturedPart struct {
	mediaType        string
	transferEncoding string
	body             []byte
}

func readQP(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(quotedprintable.NewReader(r))
	if err != nil {
		t.Fatalf("读取 quoted-printable 失败: %v", err)
	}
	return string(data)
}

func collectHTMLElements(t *testing.T, raw string) map[string]int {
	t.Helper()
	root, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("解析 HTML 失败: %v", err)
	}
	out := map[string]int{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			out[n.Data]++
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

func hasLoneLF(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			continue
		}
		if i == 0 || s[i-1] != '\r' {
			return true
		}
	}
	return false
}

func TestMultipartAlternativeMessageStructure(t *testing.T) {
	subject := strings.Repeat("中", 200)
	body := strings.Repeat("a", 10000-len("{{login_url}}")) + "{{login_url}}"
	tpl := Template{Subject: subject, Body: body}
	values := RenderValues{
		SiteName: "站点 & <b>",
		LoginURL: "https://example.invalid/a%20b?q=1&r=%2F#frag",
	}
	rendered, err := Render(TemplateWelcomeLocal, tpl, values)
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	msg, err := buildMultipartAlternativeMessage("sender@example.com", "recipient@example.com", rendered.Subject, rendered.TextBody, rendered.HTMLBody)
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("解析顶层 MIME 失败: %v", err)
	}
	if got := parsed.Header.Get("MIME-Version"); got != "1.0" {
		t.Fatalf("MIME-Version 异常: %q", got)
	}
	if got := decodeSubject(t, parsed.Header); got != subject {
		t.Fatalf("主题解码不一致: got=%q want=%q", got, subject)
	}
	topCT := parsed.Header.Get("Content-Type")
	media, params, err := mime.ParseMediaType(topCT)
	if err != nil || media != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("顶层 Content-Type 异常: %q err=%v", topCT, err)
	}
	boundary := params["boundary"]
	if !bytes.HasSuffix(msg, []byte("--"+boundary+"--\r\n")) {
		t.Fatalf("报文缺少完整结束边界: %q", msg[len(msg)-80:])
	}
	mr := multipart.NewReader(parsed.Body, boundary)
	var parts []capturedPart
	for {
		part, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("读取 multipart 失败: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("读取正文部分失败: %v", err)
		}
		parts = append(parts, capturedPart{
			mediaType:        parsePartMediaType(t, part.Header.Get("Content-Type")),
			transferEncoding: part.Header.Get("Content-Transfer-Encoding"),
			body:             body,
		})
		if len(parts) > 3 {
			t.Fatalf("正文部分不应超过 2 个")
		}
	}
	if len(parts) != 2 {
		t.Fatalf("应只有 text/plain 与 text/html 两个部分，实际 %d", len(parts))
	}
	if parts[0].mediaType != "text/plain" || parts[1].mediaType != "text/html" {
		t.Fatalf("部分顺序/类型异常: %q %q", parts[0].mediaType, parts[1].mediaType)
	}
	if parts[0].transferEncoding != "quoted-printable" || parts[1].transferEncoding != "quoted-printable" {
		t.Fatalf("传输编码应为 quoted-printable: %q %q", parts[0].transferEncoding, parts[1].transferEncoding)
	}
	textDecoded := readQP(t, bytes.NewReader(parts[0].body))
	if strings.ReplaceAll(textDecoded, "\r\n", "\n") != rendered.TextBody {
		t.Fatalf("text/plain 解码不一致: got=%q want=%q", textDecoded, rendered.TextBody)
	}
	if hasLoneLF(textDecoded) {
		t.Fatalf("text/plain 解码后仍含裸 LF: %q", textDecoded)
	}
	htmlDecoded := readQP(t, bytes.NewReader(parts[1].body))
	if strings.ReplaceAll(htmlDecoded, "\r\n", "\n") != rendered.HTMLBody {
		t.Fatalf("text/html 解码不一致: got=%q want=%q", htmlDecoded, rendered.HTMLBody)
	}
	if hasLoneLF(htmlDecoded) {
		t.Fatalf("text/html 解码后仍含裸 LF: %q", htmlDecoded)
	}
	if !strings.Contains(textDecoded, values.LoginURL) {
		t.Fatalf("纯文本 MIME 部分应含完整 URL: %q", textDecoded)
	}
	_, anchors := parseHTMLFragment(t, htmlDecoded)
	if len(anchors) != 1 || anchors[0].href != values.LoginURL || anchors[0].text != values.LoginURL {
		t.Fatalf("HTML MIME 部分锚点 href/可见文字必须与纯文本 URL 一致: %+v", anchors)
	}
	// 结构安全：只有正文两种，不得出现附件/CID/图片/脚本/远程资源。
	for _, forbidden := range []string{"Content-ID", "cid:", "Content-Disposition: attachment", "multipart/related", "multipart/mixed"} {
		if bytes.Contains(msg, []byte(forbidden)) {
			t.Fatalf("报文不得包含 %q", forbidden)
		}
	}
	elements := collectHTMLElements(t, htmlDecoded)
	for _, name := range []string{"script", "img", "iframe", "style", "link", "object", "embed", "form"} {
		if elements[name] > 0 {
			t.Fatalf("HTML 不得包含 %s 元素: %v", name, elements)
		}
	}
	if elements["a"] != 1 {
		t.Fatalf("HTML 应只有一个链接: %v", elements)
	}
}

func TestPlainMessageSinglePart(t *testing.T) {
	subject := "SMTP 配置测试"
	body := "这是一封测试邮件，收到即表示 SMTP 配置正确。\n第二行 & <x>"
	msg, err := buildPlainMessage("sender@example.com", "recipient@example.com", subject, body)
	if err != nil {
		t.Fatalf("构造纯文本报文失败: %v", err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("解析纯文本 MIME 失败: %v", err)
	}
	if got := decodeSubject(t, parsed.Header); got != subject {
		t.Fatalf("主题解码不一致: %q", got)
	}
	media, _, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || media != "text/plain" {
		t.Fatalf("纯文本 Content-Type 异常: %q err=%v", parsed.Header.Get("Content-Type"), err)
	}
	if strings.Contains(parsed.Header.Get("Content-Type"), "multipart") {
		t.Fatal("测试邮件不得使用 multipart")
	}
	decoded := readQP(t, parsed.Body)
	if strings.ReplaceAll(decoded, "\r\n", "\n") != body {
		t.Fatalf("纯文本正文解码不一致: %q", decoded)
	}
	if hasLoneLF(decoded) {
		t.Fatalf("纯文本正文应使用 CRLF: %q", decoded)
	}
}

// captureSMTPOnce 启动一次性本地 SMTP mock，捕获 DATA 阶段的完整报文。
func captureSMTPOnce(t *testing.T) (string, <-chan []byte) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	done := make(chan []byte, 1)
	go func() {
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 mock SMTP\r\n"))
		inData := false
		var data bytes.Buffer
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					_, _ = conn.Write([]byte("250 queued\r\n"))
					done <- data.Bytes()
					continue
				}
				data.WriteString(line)
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				_, _ = conn.Write([]byte("250 mock\r\n"))
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = conn.Write([]byte("250 ok\r\n"))
			case line == "DATA\r\n":
				inData = true
				_, _ = conn.Write([]byte("354 send\r\n"))
			case line == "QUIT\r\n":
				_, _ = conn.Write([]byte("221 bye\r\n"))
				return
			default:
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		}
	}()
	return listener.Addr().String(), done
}

func TestSendTestUsesFixedSinglePlainMessage(t *testing.T) {
	addr, captured := captureSMTPOnce(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	st, svc := newTestMail(t)
	ctx := context.Background()
	for k, v := range map[string]string{
		KeyHost: host, KeyPort: port, KeyFrom: "sender@example.com",
		KeySecurity: "plain", KeyAuth: "false",
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SendTest(ctx, "recipient@example.com"); err != nil {
		t.Fatalf("SendTest 失败: %v", err)
	}
	raw := <-captured
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解析测试邮件失败: %v", err)
	}
	if got := decodeSubject(t, parsed.Header); got != "SMTP 配置测试" {
		t.Fatalf("测试邮件主题应固定: %q", got)
	}
	media, _, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || media != "text/plain" {
		t.Fatalf("测试邮件应为单一 text/plain: %q err=%v", parsed.Header.Get("Content-Type"), err)
	}
	decoded := readQP(t, parsed.Body)
	// SMTP DATA 关闭阶段会追加终止用 CRLF；比较固定正文时去掉该传输层换行。
	decoded = strings.TrimSuffix(decoded, "\r\n")
	if decoded != "这是一封测试邮件，收到即表示 SMTP 配置正确。" {
		t.Fatalf("测试邮件正文应固定: %q", decoded)
	}
	_ = st
}

func TestSendMultipartThroughSMTP(t *testing.T) {
	addr, captured := captureSMTPOnce(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	_, svc := newTestMail(t)
	ctx := context.Background()
	for k, v := range map[string]string{
		KeyHost: host, KeyPort: port, KeyFrom: "sender@example.com",
		KeySecurity: "plain", KeyAuth: "false",
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	values := RenderValues{SiteName: "站点", LoginURL: "https://example.invalid/login?x=1&y=2#frag"}
	rendered, err := Render(TemplateWelcomeLocal, Template{
		Subject: "欢迎",
		Body:    "{{site_name}}\n请登录：{{login_url}}",
	}, values)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.sendMultipart(ctx, "recipient@example.com", rendered.Subject, rendered.TextBody, rendered.HTMLBody); err != nil {
		t.Fatalf("sendMultipart 失败: %v", err)
	}
	raw := <-captured
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解析 SMTP 捕获报文失败: %v", err)
	}
	media, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || media != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("业务邮件应为 multipart/alternative: %q err=%v", parsed.Header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(parsed.Body, params["boundary"])
	first, err := mr.NextRawPart()
	if err != nil {
		t.Fatalf("读取第一个正文部分失败: %v", err)
	}
	firstBody, err := io.ReadAll(first)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsePartMediaType(t, first.Header.Get("Content-Type")); got != "text/plain" {
		t.Fatalf("第一个正文部分应为 text/plain: %q", got)
	}
	decodedText := readQP(t, bytes.NewReader(firstBody))
	if !strings.Contains(decodedText, values.LoginURL) {
		t.Fatalf("实际 SMTP 报文的纯文本部分应含完整 URL: %q", decodedText)
	}
}
