package controlgate

import (
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

// selfSignedDER 造一张自签证书,返回其 DER 与解析后的 *x509.Certificate(供 pin 口径测试)。
func selfSignedDER(t *testing.T) ([]byte, *x509.Certificate) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "controlgate-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(crand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatalf("签发自签证书失败: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("解析证书失败: %v", err)
	}
	return der, cert
}

// TestSpkiPinFormat spkiPin 口径:"sha256/<base64>",且与手工按 RawSubjectPublicKeyInfo→sha256→base64 一致。
// 锁死口径:pin 计算漂移会让 NTS-KE 静默把合法服务器判为 pin 不符 → 整条抗回拨防线失效。
func TestSpkiPinFormat(t *testing.T) {
	_, cert := selfSignedDER(t)

	got := spkiPin(cert)
	if !strings.HasPrefix(got, "sha256/") {
		t.Fatalf("pin 应以 sha256/ 开头, got %q", got)
	}
	b64 := strings.TrimPrefix(got, "sha256/")
	if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
		t.Errorf("pin 主体应为合法 base64: %v", err)
	}

	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	want := "sha256/" + base64.StdEncoding.EncodeToString(sum[:])
	if got != want {
		t.Errorf("pin 口径不一致\n got=%q\nwant=%q", got, want)
	}
}

// TestPinVerifier pinVerifier 回调:pin 匹配放行、不符拒、无证书拒(全从 rawCerts[0] 自算,不依赖系统链)。
func TestPinVerifier(t *testing.T) {
	der, cert := selfSignedDER(t)
	correct := spkiPin(cert)

	// pin 匹配 → nil(放行)。
	if err := pinVerifier(correct)([][]byte{der}, nil); err != nil {
		t.Errorf("pin 匹配应放行, got err=%v", err)
	}
	// pin 不符 → error(拒)。
	if err := pinVerifier("sha256/"+base64.StdEncoding.EncodeToString(make([]byte, 32)))([][]byte{der}, nil); err == nil {
		t.Error("pin 不符应被拒")
	}
	// 无证书 → error(拒,不放行)。
	if err := pinVerifier(correct)([][]byte{}, nil); err == nil {
		t.Error("对端未提供证书应被拒")
	}
	// 无法解析的 DER → error(拒)。
	if err := pinVerifier(correct)([][]byte{[]byte("not-a-cert")}, nil); err == nil {
		t.Error("无法解析的证书应被拒")
	}
}

// TestQueryNTSRejectsNoPin srv.Pin=="" → 直接返回 error,不发起连接、不回退系统 CA。
// (甲方能污染系统信任库,无 pin 就无从只认 pin,唯有拒绝。)
func TestQueryNTSRejectsNoPin(t *testing.T) {
	// Host 是 TEST-NET-1(192.0.2.0/24,RFC 5737,保证不可路由):若真去连会超时;
	// 但无 pin 分支应在拨号前就返回 error,所以本用例必须瞬时返回。
	done := make(chan error, 1)
	go func() {
		_, err := queryNTS(context.Background(), ntsServerWire{Host: "192.0.2.1", Port: 4460, Pin: ""})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("srv.Pin=空 应返回 error(拒绝,不回退系统 CA)")
		}
		if !strings.Contains(err.Error(), "pin") {
			t.Errorf("错误应指明 pin 未配置, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("无 pin 分支应瞬时返回(未发起连接),却卡住 = 走了拨号路径")
	}
}
