package controlgate

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/beevik/nts"

	"github.com/shyandsy/aurora/logger"
)

// ntsPerServerTimeout 单台 NTS 服务器(NTS-KE + NTP 查询)整体预算上限。时钟是旁路,绝不久等。
// 实际每台用 min(本上限, ctx 剩余),ctx 到点即停(pollForTime 给整个 Now 也就 timeSourceTimeout)。
const ntsPerServerTimeout = 5 * time.Second

// defaultNtsKePort NTS-KE 默认端口(RFC 8915)。
const defaultNtsKePort = 4460

// ntsSource 是基于 NTS(RFC 8915)的可信时间源,实现 timeSource。
//
// 命根 = pin 校验:NTS-KE 的 TLS **只认服务器配置的 pin、绝不信系统 CA**(甲方是 root、能污染信任库)。
// 只接受「pin 校验通过 + NTS 认证(AES-SIV,时间在认证载荷内)」的时间;pin 不符 / 连不上 / 认证失败
// → 该服务器失败,绝不回退系统 CA、绝不接受未经认证的时间。
//
// 用 github.com/beevik/nts(RFC 8915 客户端,内部用 beevik/ntp 做 NTP + secure-io/siv-go 做 AES-SIV):
// 它 NewSessionWithOptions 只在我传入的 TLSConfig 上强制 MinVersion=TLS1.3 + ALPN=ntske/1,**不覆盖**
// InsecureSkipVerify / VerifyPeerCertificate —— 所以自定义 pin 回调会被如实沿用(见其 session.go)。
type ntsSource struct {
	name    string // 门禁名(admin/customer/schedule),日志用
	debug   bool   // 是否输出装载/取时日志(随 gate.debug)
	mu      sync.RWMutex
	servers []ntsServerWire
}

func newNtsSource(name string, debug bool) *ntsSource { return &ntsSource{name: name, debug: debug} }

// Name 源名。
func (s *ntsSource) Name() string { return "nts" }

// SetServers 原子替换服务器名单(control 每次下发裁决后调用:实时续约 + 持久化恢复两条路都灌)。
// 拷贝一份,避免与调用方共享底层数组。
func (s *ntsSource) SetServers(list []ntsServerWire) {
	cp := append([]ntsServerWire(nil), list...)
	s.mu.Lock()
	prev := len(s.servers)
	s.servers = cp
	s.mu.Unlock()
	// 名单条数变化时打一条(首次续约 0→N 即在启动后几秒出现;eng info 级可见,prd 编译期收敛)。
	if s.debug && len(cp) != prev {
		logger.Infof("controlgate[%s]: NTS 时间源装载 %d 台(control 下发)", s.name, len(cp))
	}
}

// snapshot 取一份当前名单快照(读锁,供 Now 遍历时不持锁做网络 IO)。
func (s *ntsSource) snapshot() []ntsServerWire {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ntsServerWire(nil), s.servers...)
}

// Available 是否有可试的服务器。名单为空(老 control 没下发 / 首个裁决之前)→ false → 不启用
// air-gap fail-closed(见 gate.hasAvailableSource / faultRate),避免误锁拿不到源的合法机器。
func (s *ntsSource) Available() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.servers) > 0
}

// Now 遍历(随机打散的)服务器名单,任一台成功即返回其认证 unix 秒;全失败返回 error。
// best-effort、非阻塞:只在后台 run 的 pollForTime 里被调,绝不进业务请求路径;逐台超时,ctx 到点即停。
func (s *ntsSource) Now(ctx context.Context) (int64, error) {
	list := s.snapshot()
	if len(list) == 0 {
		return 0, errors.New("nts: 无可用服务器名单")
	}
	// 打散:避免总打第一台(负载均衡 + 单台被针对时仍有机会)。
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })

	var firstErr error
	for _, srv := range list {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}
		unix, err := queryNTS(ctx, srv)
		if err == nil && unix > 0 {
			return unix, nil
		}
		if firstErr == nil {
			firstErr = err
		}
		if s.debug {
			logger.Debugf("controlgate: nts 服务器 %s 取时失败: %v", net.JoinHostPort(srv.Host, strconv.Itoa(ntsPort(srv))), err)
		}
	}
	if firstErr == nil {
		firstErr = errors.New("nts: 全部服务器失败")
	}
	return 0, firstErr
}

// ntsPort 取服务器 KE 端口,0 → 默认 4460。
func ntsPort(srv ntsServerWire) int {
	if srv.Port == 0 {
		return defaultNtsKePort
	}
	return srv.Port
}

// queryNTS 对单台服务器完整取一次 NTS 认证时间:NTS-KE(TCP,pin 校验在此)→ NTP 查询(UDP,AES-SIV 认证)。
// 返回认证得到的 unix 秒;任何环节失败(含 pin 不符)返回 error,绝不返回未经认证的时间。
func queryNTS(ctx context.Context, srv ntsServerWire) (int64, error) {
	if srv.Pin == "" {
		// 无 pin = 无法只认 pin;绝不回退系统 CA(甲方能污染信任库)→ 直接判该服务器失败。
		return 0, errors.New("nts: 服务器未配置 pin,拒绝(不回退系统 CA)")
	}

	// 每台预算 = min(上限, ctx 剩余);ctx 已到点则不发起。
	budget := ntsPerServerTimeout
	if dl, ok := ctx.Deadline(); ok {
		if rem := time.Until(dl); rem < budget {
			budget = rem
		}
	}
	if budget <= 0 {
		return 0, context.DeadlineExceeded
	}

	addr := net.JoinHostPort(srv.Host, strconv.Itoa(ntsPort(srv)))

	// ── pin-only TLS:关掉系统 CA 校验,改由 pin 回调裁决(命根)──────────────────────
	// InsecureSkipVerify=true 只是让标准链校验不跑;VerifyPeerCertificate 里做严格 pin 比对,
	// 不匹配即返回 error(TLS 握手随即失败)。绝不存在"连上但 pin 不符还接受时间"的路径。
	tlsConf := &tls.Config{
		InsecureSkipVerify:    true, //nolint:gosec // 有意:甲方是 root、系统 CA 不可信;安全性由下面的 pin 回调保证
		VerifyPeerCertificate: pinVerifier(srv.Pin),
	}

	sess, err := nts.NewSessionWithOptions(addr, &nts.SessionOptions{
		TLSConfig: tlsConf,
		Timeout:   budget, // 约束 NTS-KE(TCP)阶段
	})
	if err != nil {
		// 含 pin 不符(VerifyPeerCertificate 返回 error → 握手失败 → 这里 err)。
		return 0, fmt.Errorf("nts KE 失败(含 pin 校验): %w", err)
	}

	resp, err := sess.QueryWithOptions(&ntp.QueryOptions{Timeout: budget})
	if err != nil {
		return 0, fmt.Errorf("nts NTP 查询失败: %w", err)
	}
	if err := resp.Validate(); err != nil {
		return 0, fmt.Errorf("nts 响应无效: %w", err)
	}

	// 认证时间 = 本地时钟 + 认证得到的 ClockOffset。
	// 用 offset 而非 resp.Time:offset 抵消被甲方污染的本地墙钟(offset = 服务器时间 - 本地时间),
	// 恢复出的正是 NTS 认证过的服务器时间;RTT 也已在 offset 里修正过。
	return time.Now().Add(resp.ClockOffset).Unix(), nil
}

// pinVerifier 返回一个 tls.Config.VerifyPeerCertificate 回调:计算对端 leaf 证书的 SPKI 指纹,
// 与配置 pin 严格比对,不符即拒。计算口径与 control 侧一致:
//
//	"sha256/" + base64.StdEncoding.EncodeToString(sha256.Sum256(leaf.RawSubjectPublicKeyInfo))
//
// InsecureSkipVerify=true 时 verifiedChains 为 nil,故只从 rawCerts[0](对端 leaf)自行解析计算,
// 完全不依赖系统信任链。
func pinVerifier(pin string) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("nts pin: 对端未提供证书")
		}
		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("nts pin: 解析 leaf 证书失败: %w", err)
		}
		got := spkiPin(leaf)
		if got != pin {
			return fmt.Errorf("nts pin 不匹配: 期望 %q 实得 %q", pin, got)
		}
		return nil
	}
}

// spkiPin 计算证书的 SPKI pin(与 control 侧口径一致)。
func spkiPin(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return "sha256/" + base64.StdEncoding.EncodeToString(sum[:])
}
