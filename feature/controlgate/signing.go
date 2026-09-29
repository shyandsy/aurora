package controlgate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// signHTTPTimeout 代签请求整体超时(网络是旁路,绝不久等;失败即 fail-closed)。
const signHTTPTimeout = 10 * time.Second

// signRequest 送 control 的代签请求体。身份(project/deployment)随体带上,与 renewer 同一套坐标——
// control 据 projectUuid 选用它持有的项目私钥签名,并按 deployment 授权/吊销状态裁决是否代签。
type signRequest struct {
	Payload      string `json:"payload"`      // base64(std) 的待签原始字节
	ProjectUUID  string `json:"projectUuid"`  // 选签名私钥(control 持有,永不下发)
	DeploymentID string `json:"deploymentId"` // 授权/吊销维度
}

// signResponse control 的代签响应:sig = base64(std) 的签名字节(算法由 control + App 内置公钥约定,
// 本包不验、不关心;controlgate 只负责把 sig 取回来)。
type signResponse struct {
	Sig string `json:"sig"`
}

// SignViaControl 是 gate 对公开 Gate 契约 SignViaControl 的实现:把 payload 送 control 代签、取回签名。
//
// Phase 1 分发锁的**客户端半**:私钥永在 control(不下发、本包不持不缓存);复用 renewer 那套 control
// 认证通道坐标(RenewBaseURL + Project/Deployment 身份)向 control 的代签端点发请求。
// 授权失败 / 被吊销 / 后端不可达 / 响应异常 → 返回 error(fail-closed,**绝不返回伪造/占位签名**)。
//
// 端点与 renewer 同源同命名空间(control 已按 /projects/{proj}/deployments/{dep}/ 路由 renew,
// 代签走同一身份树):POST {RenewBaseURL}/projects/{proj}/deployments/{dep}/appconfig/sign。
// control 服务端(保管私钥 + 授权 + 吊销)是跨仓、不在本 PR;端点上线前本方法"就绪但无对端"。
func (g *gate) SignViaControl(ctx context.Context, payload []byte) ([]byte, error) {
	if g.controlBase == "" || g.projectUUID == "" || g.deploymentID == "" {
		return nil, fmt.Errorf("controlgate(%s): 缺 control 坐标,无法代签", g.name)
	}

	reqBody, err := json.Marshal(signRequest{
		Payload:      base64.StdEncoding.EncodeToString(payload),
		ProjectUUID:  g.projectUUID,
		DeploymentID: g.deploymentID,
	})
	if err != nil {
		return nil, fmt.Errorf("controlgate(%s): 代签请求序列化失败: %w", g.name, err)
	}

	url := fmt.Sprintf("%s/projects/%s/deployments/%s/appconfig/sign",
		strings.TrimRight(g.controlBase, "/"), g.projectUUID, g.deploymentID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("controlgate(%s): 构造代签请求失败: %w", g.name, err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: signHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("controlgate(%s): 代签请求失败(control 不可达): %w", g.name, err) // fail-closed
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		// 401/403 = 授权失败/被吊销;5xx = control 侧异常。一律 fail-closed,绝不占位。
		return nil, fmt.Errorf("controlgate(%s): 代签被拒 status=%d", g.name, resp.StatusCode)
	}

	var sr signResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("controlgate(%s): 代签响应解析失败: %w", g.name, err)
	}
	if sr.Sig == "" {
		return nil, fmt.Errorf("controlgate(%s): 代签响应缺 sig", g.name)
	}
	sig, err := base64.StdEncoding.DecodeString(sr.Sig)
	if err != nil {
		return nil, fmt.Errorf("controlgate(%s): 代签 sig base64 解码失败: %w", g.name, err)
	}
	if len(sig) == 0 {
		return nil, fmt.Errorf("controlgate(%s): 代签 sig 为空", g.name)
	}
	// 不验 sig:那是 App 用内置公钥的事,不是 controlgate 的 verdict 公钥。controlgate 只把 sig 取回来。
	return sig, nil
}
