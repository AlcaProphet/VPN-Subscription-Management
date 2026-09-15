package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// newSelfSignedCertificate 为 implicit TLS 隔离测试生成本地自签证书。
func newSelfSignedCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("生成测试证书序列号失败: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("签发测试证书失败: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// captureImplicitTLSOnce 启动仅监听回环地址的 implicit TLS SMTP mock，返回其地址与捕获到的命令。
func captureImplicitTLSOnce(t *testing.T) (string, <-chan string) {
	t.Helper()
	cert := newSelfSignedCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("启动 implicit TLS mock 失败: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("220 implicit TLS mock\r\n")); err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		var commands strings.Builder
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					_, _ = conn.Write([]byte("250 queued\r\n"))
				}
				continue
			}
			commands.WriteString(line)
			switch {
			case strings.HasPrefix(line, "EHLO "), strings.HasPrefix(line, "HELO "):
				_, _ = conn.Write([]byte("250 mock\r\n"))
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = conn.Write([]byte("250 ok\r\n"))
			case line == "DATA\r\n":
				inData = true
				_, _ = conn.Write([]byte("354 send\r\n"))
			case line == "QUIT\r\n":
				_, _ = conn.Write([]byte("221 bye\r\n"))
				done <- commands.String()
				return
			default:
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		}
	}()
	return listener.Addr().String(), done
}

// TestImplicitTLSConnectsWithTLSFromStart implicit TLS 必须从连接开始握手，不得先发明文 SMTP 或 STARTTLS。
func TestImplicitTLSConnectsWithTLSFromStart(t *testing.T) {
	addr, done := captureImplicitTLSOnce(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	_, svc := newTestMail(t)
	ctx := context.Background()
	for k, v := range map[string]string{
		KeyHost: host, KeyPort: port, KeyFrom: "sender@example.com",
		KeySecurity: "implicit_tls", KeyAuth: "false",
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	// 仅测试注入：信任本地自签证书，默认生产配置不受影响。
	svc.tlsConfig = &tls.Config{InsecureSkipVerify: true}
	if err := svc.SendTest(ctx, "recipient@example.com"); err != nil {
		t.Fatalf("implicit TLS 测试邮件失败: %v", err)
	}
	select {
	case commands := <-done:
		if strings.Contains(commands, "STARTTLS") {
			t.Fatalf("implicit TLS 不得再发送 STARTTLS: %q", commands)
		}
		if !strings.Contains(commands, "DATA\r\n") {
			t.Fatalf("implicit TLS 未完成 DATA 阶段: %q", commands)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 implicit TLS mock 结束超时")
	}
}
