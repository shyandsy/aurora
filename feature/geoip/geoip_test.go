package geoip

import (
	"io"
	"os"
	"reflect"
	"testing"
)

// TestNewIsNilable:New 的返回值必须是**可空类型(指针/接口)**,不能是结构体值。
// 因为 di.ProvideAs 装配时会对实例做 reflect.Value.IsNil —— 对结构体值 IsNil 直接 panic。
// 空源(降级)路径也必须返回可空类型,否则装配时会 panic。
func TestNewIsNilable(t *testing.T) {
	// 两个源都空(降级)+ 正常源两种情况都钉住返回可空。
	for _, r := range []Resolver{
		New(), // 无选项 → 两源皆空
		New(WithChina(NewIP2RegionSource("")), WithInternational(NewDBIPSource(""))),
		New(WithChina(NewIP2RegionSource("/nonexistent.xdb")), WithInternational(NewDBIPSource("/nonexistent.mmdb"))),
	} {
		v := reflect.ValueOf(r)
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Chan, reflect.Func, reflect.Slice:
			// 可空 → di.ProvideAs 的 IsNil 合法,不会 panic
		default:
			t.Fatalf("New() kind=%s 不可空,di.ProvideAs 会 IsNil panic;须返回指针/接口", v.Kind())
		}
	}
}

// TestGracefulDegradation:源路径为空 / 文件缺失时,必须优雅降级(返回空记录,不 panic)。
// 没有库也要能编译运行、绝不阻断调用。
func TestGracefulDegradation(t *testing.T) {
	r := New(WithChina(NewIP2RegionSource("")), WithInternational(NewDBIPSource("/nonexistent/path.mmdb"))) // must not panic
	if r == nil {
		t.Fatal("New returned nil")
	}
	for _, ip := range []string{"1.2.3.4", "", "not-an-ip", "8.8.8.8"} {
		if got := r.Lookup(ip); got != (Record{}) {
			t.Errorf("degraded Lookup(%q) = %+v, want empty Record", ip, got)
		}
	}
}

// TestEmbeddedIP2Region:内嵌国内库开箱即用——不配任何路径也能把国内 IP 解析出省/运营商。
// 钉住 //go:embed 的库确实被打进来且能解压查询(否则 example 的"零配置"承诺会假)。
func TestEmbeddedIP2Region(t *testing.T) {
	r := New(WithChina(EmbeddedIP2RegionSource())) // 只用内嵌国内源,不配任何文件
	got := r.Lookup("112.224.163.187")             // 已知国内 IP(山东 · 联通)
	if !got.IsChina() {
		t.Fatalf("内嵌 ip2region 应把国内 IP 判为 CN,got=%+v", got)
	}
	if got.Province == "" {
		t.Fatalf("内嵌 ip2region 应解出省份,got=%+v", got)
	}
}

// TestEmbeddedDBIP:内嵌国外库开箱即用——不配任何路径也能把国外 IP 解析出国家。
// 同时钉住内嵌 DB-IP 的临时文件生命周期:Close 后临时文件必须被删掉(不泄漏)。
func TestEmbeddedDBIP(t *testing.T) {
	src := EmbeddedDBIPSource() // 只用内嵌国外源,不配任何文件
	closer, ok := src.(interface {
		Source
		io.Closer
	})
	if !ok {
		t.Fatalf("内嵌 DB-IP 源应实现 io.Closer,got %T(可能是解压/落盘失败降级为空源)", src)
	}

	r := New(WithInternational(src))
	got := r.Lookup("8.8.8.8") // 已知国外 IP(Google DNS,US)
	if got.CountryISO != "US" {
		t.Fatalf("内嵌 DB-IP 应把 8.8.8.8 解析为 US,got=%+v", got)
	}

	// Close 应删掉解压出的临时文件。取内部 tmpPath 校验其已不存在。
	tmpPath := src.(*dbipSource).tmpPath
	if tmpPath == "" {
		t.Fatal("内嵌 DB-IP 源应带临时文件路径")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Close 后临时文件应被删除,但 stat(%q) err=%v(非 NotExist)", tmpPath, err)
	}
}

// fakeSource 固定返回一条记录,用于钉住 router 的分流/兜底逻辑(不依赖真库文件)。
type fakeSource struct{ rec Record }

func (f fakeSource) Lookup(string) Record { return f.rec }
func (fakeSource) Name() string           { return "fake" }

// TestChinaFallback:国内「有省无市」时,WithChinaFallback 开关 + 省级闸门的行为。
func TestChinaFallback(t *testing.T) {
	cnProvinceOnly := Record{CountryISO: "CN", Province: "山东省", City: "", ISP: "联通"} // ip2region 只到省
	intlMatch := Record{CountryISO: "CN", Province: "Shandong", City: "Jinan"}       // DB-IP 省一致
	intlMismatch := Record{CountryISO: "CN", Province: "Zhejiang", City: "Hangzhou"} // DB-IP 省不一致

	t.Run("开关关:不兜底,保持只到省", func(t *testing.T) {
		r := New(WithChina(fakeSource{cnProvinceOnly}), WithInternational(fakeSource{intlMatch}))
		got := r.Lookup("1.2.3.4")
		if got.City != "" || got.CityApprox {
			t.Fatalf("关闭兜底不应补市,got=%+v", got)
		}
	})
	t.Run("开+省一致:补参考市并映射中文+标 approx", func(t *testing.T) {
		r := New(WithChina(fakeSource{cnProvinceOnly}), WithInternational(fakeSource{intlMatch}), WithChinaFallback(true))
		got := r.Lookup("1.2.3.4")
		if got.City != "济南市" || !got.CityApprox || got.Province != "山东省" || got.ISP != "联通" {
			t.Fatalf("省一致应补「济南市」并标 approx,got=%+v", got)
		}
	})
	t.Run("开+省不一致:不采信,保持只到省", func(t *testing.T) {
		r := New(WithChina(fakeSource{cnProvinceOnly}), WithInternational(fakeSource{intlMismatch}), WithChinaFallback(true))
		got := r.Lookup("1.2.3.4")
		if got.City != "" || got.CityApprox {
			t.Fatalf("省不一致不应补市,got=%+v", got)
		}
	})
	t.Run("ip2region 已有市:开着也不动它", func(t *testing.T) {
		cnFull := Record{CountryISO: "CN", Province: "广东省", City: "深圳市", ISP: "电信"}
		r := New(WithChina(fakeSource{cnFull}), WithInternational(fakeSource{intlMatch}), WithChinaFallback(true))
		got := r.Lookup("1.2.3.4")
		if got.City != "深圳市" || got.CityApprox {
			t.Fatalf("已有市不应被覆盖,got=%+v", got)
		}
	})
}

// TestEmbeddedASN:内嵌 ASN 库开箱即用——解析已知 IP 的 ASN/组织名并按组织名判机房;Close 删临时文件。
func TestEmbeddedASN(t *testing.T) {
	src := EmbeddedDBIPASNSource()
	closer, ok := src.(interface {
		Source
		io.Closer
	})
	if !ok {
		t.Fatalf("内嵌 ASN 源应实现 io.Closer,got %T(可能解压/落盘失败降级为空源)", src)
	}
	got := src.Lookup("8.8.8.8") // Google DNS,AS15169
	if got.ASN != 15169 {
		t.Fatalf("8.8.8.8 应为 AS15169,got ASN=%d org=%q", got.ASN, got.ASNOrg)
	}
	if !got.IsHosting {
		t.Fatalf("Google ASN 应判为机房/云,got=%+v", got)
	}
	tmpPath := src.(*dbipASNSource).tmpPath
	if tmpPath == "" {
		t.Fatal("内嵌 ASN 源应带临时文件路径")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Close 后临时文件应被删除,stat(%q) err=%v(非 NotExist)", tmpPath, err)
	}
}

// TestASNToggleAndMerge:ASN 是**可开关**的(WithASN 传了才查、不写死),且与归属地**正交**——
// 并进同一 Record 不影响定位字段;国内 IP 也查(国内云=机房)。
func TestASNToggleAndMerge(t *testing.T) {
	geo := fakeSource{Record{CountryISO: "US", Province: "California", City: "Mountain View"}}
	asn := fakeSource{Record{ASN: 15169, ASNOrg: "Google LLC", IsHosting: true}}

	t.Run("不传 WithASN:ASN 字段保持零值(未启用)", func(t *testing.T) {
		got := New(WithInternational(geo)).Lookup("8.8.8.8")
		if got.ASN != 0 || got.ASNOrg != "" || got.IsHosting {
			t.Fatalf("未启用 ASN 时不应有 ASN 结论,got=%+v", got)
		}
		if got.CountryISO != "US" || got.City != "Mountain View" {
			t.Fatalf("定位字段应正常,got=%+v", got)
		}
	})
	t.Run("传 WithASN:ASN 并进结果,定位字段不受影响", func(t *testing.T) {
		got := New(WithInternational(geo), WithASN(asn)).Lookup("8.8.8.8")
		if got.ASN != 15169 || got.ASNOrg != "Google LLC" || !got.IsHosting {
			t.Fatalf("应并入 ASN 结论,got=%+v", got)
		}
		if got.CountryISO != "US" || got.City != "Mountain View" {
			t.Fatalf("ASN 增强不应改动定位字段,got=%+v", got)
		}
	})
	t.Run("国内 IP 也查 ASN(国内云=机房)", func(t *testing.T) {
		cn := fakeSource{Record{CountryISO: "CN", Province: "北京市", ISP: "阿里云"}}
		cloud := fakeSource{Record{ASN: 37963, ASNOrg: "Alibaba", IsHosting: true}}
		got := New(WithChina(cn), WithASN(cloud)).Lookup("1.2.3.4")
		if !got.IsChina() || got.ASN != 37963 || !got.IsHosting {
			t.Fatalf("国内 IP 也应带 ASN 结论,got=%+v", got)
		}
	})
}

// TestLooksLikeHosting:机房/云/Tor 组织名判 true;住宅/骨干运营商判 false;空串 false。启发式钉住。
func TestLooksLikeHosting(t *testing.T) {
	hosting := []string{
		"Google LLC", "Cloudflare, Inc.", "The Infrastructure Group B.V.",
		"Foreningen for digitala fri- och rattigheter", // DFRI(Tor 出口)
		"Amazon.com, Inc.", "DigitalOcean, LLC", "OVH SAS", "Hetzner Online GmbH",
	}
	residential := []string{
		"CHINA UNICOM China169 Backbone", "China Telecom", "Comcast Cable Communications", "",
	}
	for _, o := range hosting {
		if !looksLikeHosting(o) {
			t.Errorf("looksLikeHosting(%q) = false, want true", o)
		}
	}
	for _, o := range residential {
		if looksLikeHosting(o) {
			t.Errorf("looksLikeHosting(%q) = true, want false", o)
		}
	}
}
