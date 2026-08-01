package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	RunLevelLocal      = "local"
	RunLevelEng        = "eng"
	RunLevelStage      = "stage"
	RunLevelProduction = "production"
)

// TrustedProxiesNone 是 TRUSTED_PROXIES 的显式哨兵值:谁都不信任,
// c.ClientIP() 一律返回直连对端 IP、绝不采信任何 X-Forwarded-For。
const TrustedProxiesNone = "none"

// DefaultTrustedProxies 是 TRUSTED_PROXIES 未设置时的默认可信代理网段:
// loopback + RFC1918 私网 + IPv6 loopback/ULA。它匹配 aurora 服务最常见的部署形态
// ——服务只经私有网(容器网 / 内网)里的反向代理可达。于是:来自反代(私网 IP)的请求
// 才解析 X-Forwarded-For 拿到真实客户端 IP;直连公网的请求不被信任、无法伪造 XFF。
var DefaultTrustedProxies = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

var ValidRunLevels = map[string]bool{
	RunLevelLocal:      true,
	RunLevelEng:        true,
	RunLevelStage:      true,
	RunLevelProduction: true,
}

func validRunLevelsString() string {
	return fmt.Sprintf("%s, %s, %s, %s", RunLevelLocal, RunLevelEng, RunLevelStage, RunLevelProduction)
}

type ServerConfig struct {
	Host            string        `env:"HOST" envDefault:"0.0.0.0"`
	Port            int           `env:"PORT" envDefault:"8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"5s"`

	Name     string `env:"SERVICE_NAME" envDefault:"myapp"`
	Version  string `env:"SERVICE_VERSION" envDefault:"1.0.0"`
	RunLevel string `env:"RUN_LEVEL" envDefault:"local"`

	// TrustedProxies 决定 gin 采信哪些代理转发的 X-Forwarded-For / X-Real-IP,
	// 即 c.ClientIP() 拿到的到底是真实客户端 IP 还是可被伪造的值。只有当请求的**直接对端**
	// 落在这些网段内时,gin 才解析 XFF;否则 c.ClientIP() 用直连对端 IP。
	//
	//   - 不设(空)        → 用 DefaultTrustedProxies(loopback + RFC1918 私网),
	//                        开箱即"反代后拿真 IP + 直连公网无法伪造"。
	//   - "none"           → 谁都不信,c.ClientIP() 恒为直连对端。
	//   - "0.0.0.0/0,::/0" → 信任所有代理(gin 历史默认,可被伪造 XFF,不推荐)。
	//   - "<cidr,...>"     → 精确信任这些 IP/CIDR(如只信你的反代地址)。
	//
	// 注意:相对 gin 默认("信任所有代理")这是一次安全硬化,会影响所有读 c.ClientIP() 的代码。
	// 用 ,omitempty(可选)+ 默认值写在代码里(见 DefaultTrustedProxies)—— envDefault 在本框架不生效。
	TrustedProxies []string `env:"TRUSTED_PROXIES,omitempty"`
}

func (s *ServerConfig) Key() string {
	return "server"
}

func (s *ServerConfig) Validate() error {
	ip := net.ParseIP(s.Host)
	if ip == nil {
		return NewConfigError("HOST should be a valid IP address")
	}

	if s.Port < 1 || s.Port > 65535 {
		return NewConfigError("PORT should be in [1, 65535]")
	}

	if s.Name == "" {
		return NewConfigError("SERVICE_NAME is required")
	}

	if !ValidRunLevels[s.RunLevel] {
		return NewConfigError(fmt.Sprintf("RUN_LEVEL must be one of: %s", validRunLevelsString()))
	}

	// 可信代理:对解析出的最终网段逐个校验(非法 IP/CIDR 直接 fail-fast)。
	for _, p := range s.ResolvedTrustedProxies() {
		if !isValidProxyEntry(p) {
			return NewConfigError(fmt.Sprintf("TRUSTED_PROXIES contains an invalid IP or CIDR: %q", p))
		}
	}

	return nil
}

// ResolvedTrustedProxies 把 TrustedProxies 解析成最终传给 gin SetTrustedProxies 的网段列表:
// 未设 → DefaultTrustedProxies(私网);单个 "none" 哨兵 → 空(谁都不信);否则原样返回。
func (s *ServerConfig) ResolvedTrustedProxies() []string {
	if len(s.TrustedProxies) == 0 {
		return DefaultTrustedProxies
	}
	if len(s.TrustedProxies) == 1 && strings.EqualFold(strings.TrimSpace(s.TrustedProxies[0]), TrustedProxiesNone) {
		return []string{}
	}
	return s.TrustedProxies
}

// isValidProxyEntry 接受 CIDR(如 10.0.0.0/8)或裸 IP(如 1.2.3.4)—— 与 gin SetTrustedProxies 的接受面一致。
func isValidProxyEntry(entry string) bool {
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}
	return net.ParseIP(entry) != nil
}

func (s *ServerConfig) IsProduction() bool {
	return s.RunLevel == RunLevelProduction
}

func (s *ServerConfig) GinMode() string {
	if s.IsProduction() {
		return "release"
	}
	return "debug"
}
