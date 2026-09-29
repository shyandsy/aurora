package controlgate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newSignGate 造一个只配了 control 坐标的 gate(SignViaControl 不碰时钟/门禁状态)。
func newSignGate(base string) *gate {
	return &gate{name: "t", controlBase: base, projectUUID: "p", deploymentID: "d"}
}

// TestSignViaControlHappyPath 假 control 代签成功 → 原样返回 sig;并校验请求路径 + 身份 + payload。
func TestSignViaControlHappyPath(t *testing.T) {
	payload := []byte("app.json raw bytes \x00\x01\x02")
	wantSig := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x99}

	var gotPath string
	var gotReq signRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := readAll(r)
		_ = json.Unmarshal(body, &gotReq)
		_ = json.NewEncoder(w).Encode(signResponse{Sig: base64.StdEncoding.EncodeToString(wantSig)})
	}))
	defer srv.Close()

	g := newSignGate(srv.URL)
	sig, err := g.SignViaControl(context.Background(), payload)
	if err != nil {
		t.Fatalf("代签应成功: %v", err)
	}
	if !bytes.Equal(sig, wantSig) {
		t.Errorf("应原样返回 control 的 sig, got %x want %x", sig, wantSig)
	}

	// 路径与 renewer 同一身份命名空间。
	if gotPath != "/projects/p/deployments/d/appconfig/sign" {
		t.Errorf("代签端点路径不对: %q", gotPath)
	}
	// 身份随体带上,payload 是 base64(原始字节)。
	if gotReq.ProjectUUID != "p" || gotReq.DeploymentID != "d" {
		t.Errorf("请求应带 project/deployment 身份, got %+v", gotReq)
	}
	decoded, err := base64.StdEncoding.DecodeString(gotReq.Payload)
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Errorf("payload 应为 base64(原始字节), decoded=%x err=%v", decoded, err)
	}
}

// TestSignViaControlRejectedFailsClosed control 返 401/非 200 → 返回 error 且 sig 为 nil(fail-closed)。
func TestSignViaControlRejectedFailsClosed(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"sig":"QUJD"}`)) // 即便带了 sig,非 200 也绝不采信
		}))
		g := newSignGate(srv.URL)
		sig, err := g.SignViaControl(context.Background(), []byte("x"))
		srv.Close()
		if err == nil {
			t.Errorf("status=%d 应返回 error", code)
		}
		if sig != nil {
			t.Errorf("status=%d 应 fail-closed(sig=nil), got %x", code, sig)
		}
	}
}

// TestSignViaControlUnreachableFailsClosed control 不可达 → 返回 error。
func TestSignViaControlUnreachableFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 立刻关掉:连接被拒 → Do 返回错误

	g := newSignGate(url)
	sig, err := g.SignViaControl(context.Background(), []byte("x"))
	if err == nil {
		t.Error("control 不可达应返回 error")
	}
	if sig != nil {
		t.Errorf("不可达应 fail-closed(sig=nil), got %x", sig)
	}
}

// TestSignViaControlBadResponse 空 sig / 坏 base64 → error(不返回垃圾字节)。
func TestSignViaControlBadResponse(t *testing.T) {
	cases := map[string]string{
		"空 sig":     `{"sig":""}`,
		"坏 base64":  `{"sig":"@@@not-base64@@@"}`,
		"非 JSON 响应": `not json`,
	}
	for name, resp := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(resp))
		}))
		g := newSignGate(srv.URL)
		sig, err := g.SignViaControl(context.Background(), []byte("x"))
		srv.Close()
		if err == nil || sig != nil {
			t.Errorf("%s: 应返回 error 且 sig=nil, got sig=%x err=%v", name, sig, err)
		}
	}
}

// TestSignViaControlMissingCoords 缺坐标 → error(不发请求)。
func TestSignViaControlMissingCoords(t *testing.T) {
	g := &gate{name: "t"} // 无 controlBase/proj/dep
	if _, err := g.SignViaControl(context.Background(), []byte("x")); err == nil {
		t.Error("缺 control 坐标应返回 error")
	}
}

// TestDisabledGateSignViaControl 门禁关闭 → 明确 error,不 panic。
func TestDisabledGateSignViaControl(t *testing.T) {
	var g Gate = disabledGate{}
	sig, err := g.SignViaControl(context.Background(), []byte("x"))
	if err == nil {
		t.Error("disabledGate 代签应返回 error")
	}
	if sig != nil {
		t.Errorf("disabledGate 应无 sig, got %x", sig)
	}
	if !strings.Contains(err.Error(), "代签") && !strings.Contains(err.Error(), "SignViaControl") {
		t.Errorf("error 应说明无法代签, got %v", err)
	}
}

// readAll 读请求体(测试小工具)。
func readAll(r *http.Request) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}
