// Package geoip 做「IP → 归属地」的**本地离线**解析,单一出口(收口)。
//
// 设计:调用方只认一个方法 `Resolver.Lookup(ip) Record`,直接拿归属地;**路由策略在构造时定死**——
// 传入两个源:china 源管中国大陆、international 源管其余。判定与分流全在内部,调用方无感知。
//
//	r := geoip.New(
//	    geoip.WithChina(geoip.NewIP2RegionSource(ip2regionPath)), // 国内:细到省/市 + 运营商,中文
//	    geoip.WithInternational(geoip.NewDBIPSource(dbipPath)),   // 国外:省/州 + 市(英文)
//	)
//	rec := r.Lookup(ip) // rec.CountryISO / Province / City / ISP
//
// 两个源都免费可再分发(ip2region=Apache 2.0、DB-IP=CC-BY),适合随镜像交付。
// 优雅降级:某个源路径为空/打开失败 → 该源返回空记录,不 panic、不阻断调用。IP 只查本地库,绝不外发。
//
// 另有**可选**的 ASN 面(与归属地正交,回答「谁家的网 / 是不是机房」):传 WithASN(或 Feature 层
// WithASNEnabled(true))才启用,给 Record 补 ASN/ASNOrg/IsHosting——用于识别「注册自机房/云/Tor 出口」这类信号。
// 默认不开、不加载 ASN 库。
//
// 作为 aurora Feature 使用见 NewFeature / Config(读 GEOIP_IP2REGION_PATH、GEOIP_DBIP_PATH、GEOIP_ASN_PATH)。
package geoip

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	maxminddb "github.com/oschwald/maxminddb-golang"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
)

// embeddedIP2RegionGz 是**编译期内嵌**的国内库 ip2region_v4.xdb(gz 压缩,~4MB)。
// 随二进制交付 → 国内解析开箱即用、零配置。ip2region 本就整库读进 []byte,内嵌不增运行时 RAM,
// 仅让二进制大 ~4MB。
//
//go:embed data/ip2region_v4.xdb.gz
var embeddedIP2RegionGz []byte

// embeddedDBIPGz 是**编译期内嵌**的国外库 DB-IP City Lite(mmdb,gz 压缩,~57MB)。
// 随二进制交付 → 国外解析也开箱即用、零配置、构建不联网(实体 gz 在版本控制里,防上游闭源)。
// 解压后 ~121MB:不常驻内存,而是解压到临时文件后 maxminddb.Open 走 **mmap**(冷内存),见 EmbeddedDBIPSource。
//
//go:embed data/dbip-city-lite.mmdb.gz
var embeddedDBIPGz []byte

// embeddedDBIPASNGz 是**编译期内嵌**的 DB-IP ASN Lite(mmdb,gz 压缩,~5MB)。
// 提供 IP → ASN(自治系统号 + 归属组织名),与归属地(在哪)正交:它回答「这个 IP 是谁家的网」,
// 用来识别「注册自机房/云/Tor 出口」这类信号。库与 DB-IP City 同源同许可(CC-BY),schema
// 兼容 GeoLite2-ASN。随二进制交付、构建不联网(实体 gz 在版本控制里)。解压后 ~9.5MB:
// 与 DB-IP City 一样不常驻,解压到临时文件后 mmap(见 EmbeddedDBIPASNSource)。
// **是否启用由调用方决定**(WithASN / 在 Feature 层 WithASNEnabled),默认不开、不加载。
//
//go:embed data/dbip-asn-lite.mmdb.gz
var embeddedDBIPASNGz []byte

// Record 一条归属地记录。未知/空字段统一为 ""。
type Record struct {
	CountryISO string // ISO-3166 alpha-2(CN/US/…);判定国内外的唯一依据
	Province   string // 省 / 州
	City       string // 市
	ISP        string // 运营商(仅国内源有:电信/联通/移动…)
	// CityApprox 该「市」是否为**参考值**:国内(CN)ip2region 只到省、无市时,
	// 开启 WithChinaFallback 后用国外库(DB-IP)在**省一致**前提下兜底补的市。
	// DB-IP 对华城市偏粗(精度有限)→ 标记为参考,展示侧应提示「参考」,分析侧应知其非权威。
	CityApprox bool
	// ── ASN 面(与归属地正交,仅当启用 ASN 源时填;回答「谁家的网 / 是不是机房」)──
	ASN    uint   // 自治系统号(如 15169);0 = 未知/未启用/私网。
	ASNOrg string // AS 归属组织名(如 "Google LLC"、"CHINA UNICOM China169 Backbone");"" = 未知。
	// IsHosting 该 ASN 是否**机房/云/托管/Tor 出口**这类「非终端用户网络」——由 ASNOrg 关键词启发式判定。
	// 是**结论性**字段:true 表示这个 IP 大概率来自数据中心而非住宅/移动宽带,是识别脚本/机器人注册的硬信号之一。
	// ⚠️ 启发式,非权威(无免费权威的 hosting 数据源):可能漏判小众机房、误判个别名字含关键词的运营商;够用不完美。
	IsHosting bool
}

// IsChina 是否判为中国大陆。
func (r Record) IsChina() bool { return r.CountryISO == "CN" }

// Source 单个 IP 库(ip2region / DB-IP 各实现一个)。查不到返回零值 Record。
type Source interface {
	Lookup(ip string) Record
	Name() string
}

// Resolver 收口出口:调用方唯一入口。
type Resolver interface {
	// Lookup 传 IP 直接拿归属地。内部按构造时的策略分流(国内源 / 国外源)。
	Lookup(ip string) Record
}

// Option 配置项(功能选项模式)。用 With* 指定各源,调用处一眼看清谁管国内、谁管国外。
type Option func(*resolverConfig)

type resolverConfig struct {
	china         Source
	international Source
	asn           Source // ASN 源(可选,与归属地正交);nil = 不查 ASN
	cnFallback    bool
	asnEnabled    bool // 仅供 Feature 层读:是否让 NewFeature 装配内嵌/外挂 ASN 源(见 WithASNEnabled)
}

// WithChina 指定管**中国大陆**的源(通常 ip2region)。它也负责判定 IP 是否国内(CountryISO=="CN")。
func WithChina(s Source) Option { return func(c *resolverConfig) { c.china = s } }

// WithInternational 指定管**国外**的源(通常 DB-IP)。
func WithInternational(s Source) Option { return func(c *resolverConfig) { c.international = s } }

// WithASN 指定 ASN 源(通常 DB-IP ASN Lite)。**这是库层的开关:传了才查 ASN、不传就完全不查**
// (Record 的 ASN/ASNOrg/IsHosting 留零值)——ASN 与国内/国外定位正交,查到就把三个字段并进结果。
// 裸用底层库时:geoip.New(WithChina(...), WithInternational(...), WithASN(geoip.EmbeddedDBIPASNSource()))。
// 走 Feature 时改用 WithASNEnabled(true)(由 Feature 负责按 env/内嵌装配源)。
func WithASN(s Source) Option { return func(c *resolverConfig) { c.asn = s } }

// WithASNEnabled 是**Feature 层**的 ASN 开关(默认关):传 true 时 NewFeature 才装配 ASN 源
// (GEOIP_ASN_PATH 有值→外挂;否则→内嵌库),否则完全不加载 ASN 库、不查 ASN。**不写死,交给装配处决定**:
// app.AddFeature(geoip.NewFeature(geoip.WithASNEnabled(true)))。
func WithASNEnabled(on bool) Option { return func(c *resolverConfig) { c.asnEnabled = on } }

// WithChinaFallback 开关(库参数,由调用方决定):国内(CN)命中但字段不全(目前是「只到省、无市」)时,
// 是否用国外库(DB-IP)在**省一致**前提下兜底补全,并把补来的市标记为参考(CityApprox=true)。
// 关(默认):严格只用 ip2region,缺就缺。开:多补一个精度有限的参考市。
// 是收口库的通用能力,是否启用交给装配处传(如 NewFeature(WithChinaFallback(true)))。
func WithChinaFallback(on bool) Option { return func(c *resolverConfig) { c.cnFallback = on } }

// New 构造收口解析器。用 With* 选项指定各源,一眼看清语义:
//
//	geoip.New(
//	    geoip.WithChina(geoip.NewIP2RegionSource(ip2Path)),      // 国内:中文省市 + 运营商
//	    geoip.WithInternational(geoip.NewDBIPSource(dbipPath)),  // 国外:英文省市
//	)
//
// 路由:先用 china 源查;若判为 CN → 用它;否则用 international 源。
// 未指定的源退化成空源(该地区解析返回空,不 panic、不阻断调用)。
func New(opts ...Option) Resolver {
	c := &resolverConfig{china: nopSource{}, international: nopSource{}}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	if c.china == nil {
		c.china = nopSource{}
	}
	if c.international == nil {
		c.international = nopSource{}
	}
	return &router{cn: c.china, intl: c.international, asn: c.asn, cnFallback: c.cnFallback}
}

type router struct {
	cn         Source
	intl       Source
	asn        Source // nil = 不查 ASN(未启用)
	cnFallback bool
}

// Lookup 见 Resolver。非法/空 IP → 空 Record。
func (r *router) Lookup(ip string) Record {
	ip = strings.TrimSpace(ip)
	if ip == "" || net.ParseIP(ip) == nil {
		return Record{}
	}
	// ① 定位(国内/国外二选一):决定 CountryISO/Province/City/ISP。
	var rec Record
	if cn := r.cn.Lookup(ip); cn.IsChina() {
		rec = cn
		// 国内 → 国内源(中文省/市 + 运营商)。开了兜底且「有省无市」时,用国外库补一个参考市:
		// 仅当国外库的省(转中文后)与 ip2region 的省一致才采信(挡掉 DB-IP 跨省乱标),标 CityApprox。
		if r.cnFallback && rec.City == "" && rec.Province != "" {
			if intl := r.intl.Lookup(ip); intl.City != "" && provinceCN(intl.Province) == rec.Province {
				rec.City = cityCN(intl.City)
				rec.CityApprox = true
			}
		}
	} else {
		rec = r.intl.Lookup(ip) // 国外(或国内源没命中)→ 国外源
	}
	// ② ASN 增强(与①正交,恒查):启用了 ASN 源就补 ASN/ASNOrg/IsHosting——国内 IP 也查(国内云主机=机房)。
	if r.asn != nil {
		a := r.asn.Lookup(ip)
		rec.ASN, rec.ASNOrg, rec.IsHosting = a.ASN, a.ASNOrg, a.IsHosting
	}
	return rec
}

// ── 空源 ─────────────────────────────────────────────────────────────────────
type nopSource struct{}

func (nopSource) Lookup(string) Record { return Record{} }
func (nopSource) Name() string         { return "nop" }

// ── ip2region 源(国内细粒度,中文 + 运营商)──────────────────────────────────
type ip2regionSource struct {
	buff []byte // 整库载内存(~11MB);每次查从共享 buffer 建轻量 searcher,并发安全
}

// NewIP2RegionSource 载入 ip2region_v4.xdb。路径空/读失败 → 空源(返回零值 Record)。
func NewIP2RegionSource(path string) Source {
	if path == "" {
		logger.Infof("geoip: 未配 ip2region(路径空),国内源禁用")
		return nopSource{}
	}
	buff, err := os.ReadFile(path)
	if err != nil {
		logger.Errorf("geoip: ip2region 读取失败 %s,国内源禁用: %+v", path, err)
		return nopSource{}
	}
	logger.Infof("geoip: ip2region 已载入内存 %s (%d bytes)", path, len(buff))
	return &ip2regionSource{buff: buff}
}

// EmbeddedIP2RegionSource 用**编译期内嵌**的 ip2region_v4.xdb(见 embeddedIP2RegionGz)构造国内源,
// 零配置开箱即用:内存 gunzip 后与读文件等价,**不落任何临时文件**。解压失败 → 空源(降级,不 panic)。
// 直接用底层库(不走 Feature)且想开箱即用时用它:geoip.New(geoip.WithChina(geoip.EmbeddedIP2RegionSource()))。
func EmbeddedIP2RegionSource() Source {
	gz, err := gzip.NewReader(bytes.NewReader(embeddedIP2RegionGz))
	if err != nil {
		logger.Errorf("geoip: 内嵌 ip2region 解压器创建失败,国内源禁用: %+v", err)
		return nopSource{}
	}
	defer gz.Close()
	buff, err := io.ReadAll(gz)
	if err != nil {
		logger.Errorf("geoip: 内嵌 ip2region 解压失败,国内源禁用: %+v", err)
		return nopSource{}
	}
	logger.Infof("geoip: 内嵌 ip2region 已解压载入内存 (%d bytes)", len(buff))
	return &ip2regionSource{buff: buff}
}

// resolveChinaSource 决定国内源:环境变量 GEOIP_IP2REGION_PATH 有值 → 用该文件(覆盖,便于线下更新库);
// 否则 → 用内嵌 gz(开箱即用)。
func resolveChinaSource(path string) Source {
	if path != "" {
		logger.Infof("geoip: GEOIP_IP2REGION_PATH 已配,国内源用外挂文件覆盖内嵌库: %s", path)
		return NewIP2RegionSource(path)
	}
	return EmbeddedIP2RegionSource()
}

func (ip2regionSource) Name() string { return "ip2region" }

// Lookup 解析 "国家|省|市|ISP|ISO"(v4 五段)。"0"/"" 归一为 ""。
func (s *ip2regionSource) Lookup(ip string) Record {
	searcher, err := xdb.NewWithBuffer(xdb.IPv4, s.buff)
	if err != nil {
		return Record{}
	}
	defer searcher.Close()
	raw, err := searcher.Search(ip)
	if err != nil || raw == "" {
		return Record{}
	}
	parts := strings.Split(raw, "|")
	field := func(i int) string {
		if i >= len(parts) {
			return ""
		}
		if v := strings.TrimSpace(parts[i]); v != "0" {
			return v
		}
		return ""
	}
	return Record{
		CountryISO: field(4),
		Province:   field(1),
		City:       field(2),
		ISP:        field(3),
	}
}

// ── DB-IP / GeoLite2 City 源(国外,英文省/市)────────────────────────────────
type dbipSource struct {
	reader  *maxminddb.Reader // mmap,并发安全
	tmpPath string            // 非空 = 来自内嵌库、解压到的临时文件,Close 时要一并删除;外挂文件为空(不删用户的库)
}

// NewDBIPSource 打开 DB-IP City mmdb(外挂文件,直接 mmap)。路径空/打开失败 → 空源。
// 返回的非空源额外实现 io.Closer:Feature.Close 用它释放底层 mmap reader。
func NewDBIPSource(path string) Source {
	if path == "" {
		logger.Infof("geoip: 未配 DB-IP(路径空),国外源禁用")
		return nopSource{}
	}
	reader, err := maxminddb.Open(path)
	if err != nil {
		logger.Errorf("geoip: DB-IP 打开失败 %s,国外源禁用: %+v", path, err)
		return nopSource{}
	}
	logger.Infof("geoip: DB-IP 已加载 %s (type=%s)", path, reader.Metadata.DatabaseType)
	return &dbipSource{reader: reader} // 外挂:tmpPath 空,Close 不删文件
}

// EmbeddedDBIPSource 用**编译期内嵌**的 DB-IP 库(见 embeddedDBIPGz)构造国外源,零配置开箱即用。
// 库解压后 ~121MB,为省内存**不常驻**:内存 gunzip → 写到临时文件 → maxminddb.Open 走 **mmap**(冷内存,
// 按需分页)。需要一个可写的临时目录(os.TempDir)。返回源实现 io.Closer:Close 时关 reader **并删临时文件**。
// 任何失败(解压 / 落盘 / 打开)→ 空源(降级,不 panic);已落盘但后续失败时会把临时文件清掉,不泄漏。
func EmbeddedDBIPSource() Source {
	gz, err := gzip.NewReader(bytes.NewReader(embeddedDBIPGz))
	if err != nil {
		logger.Errorf("geoip: 内嵌 DB-IP 解压器创建失败,国外源禁用: %+v", err)
		return nopSource{}
	}
	defer gz.Close()

	tmp, err := os.CreateTemp("", "geoip-dbip-*.mmdb")
	if err != nil {
		logger.Errorf("geoip: 内嵌 DB-IP 临时文件创建失败,国外源禁用: %+v", err)
		return nopSource{}
	}
	tmpPath := tmp.Name()

	// 解压落盘;任何失败都把已建的临时文件清掉,不泄漏。
	if _, err := io.Copy(tmp, gz); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP 解压落盘失败,国外源禁用: %+v", err)
		return nopSource{}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP 临时文件关闭失败,国外源禁用: %+v", err)
		return nopSource{}
	}

	reader, err := maxminddb.Open(tmpPath) // mmap;临时文件在 Close 前需一直存在
	if err != nil {
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP 打开失败,国外源禁用: %+v", err)
		return nopSource{}
	}
	logger.Infof("geoip: 内嵌 DB-IP 已解压到临时文件并 mmap %s (type=%s)", tmpPath, reader.Metadata.DatabaseType)
	return &dbipSource{reader: reader, tmpPath: tmpPath}
}

// resolveIntlSource 决定国外源:环境变量 GEOIP_DBIP_PATH 有值 → 用该文件(覆盖,直接 mmap);
// 否则 → 用内嵌 gz(解压到临时文件后 mmap)。
func resolveIntlSource(path string) Source {
	if path != "" {
		logger.Infof("geoip: GEOIP_DBIP_PATH 已配,国外源用外挂文件覆盖内嵌库: %s", path)
		return NewDBIPSource(path)
	}
	return EmbeddedDBIPSource()
}

func (dbipSource) Name() string { return "dbip" }

// Close 释放底层 mmap reader;若源来自内嵌库(tmpPath 非空),再删掉解压出的临时文件。
// 实现 io.Closer,供 Feature.Close 关闭(降级为空源时不实现,无需关)。
func (s *dbipSource) Close() error {
	err := s.reader.Close()
	if s.tmpPath != "" {
		if rmErr := os.Remove(s.tmpPath); rmErr != nil && err == nil {
			err = rmErr
		}
	}
	return err
}

type dbipRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

func (s *dbipSource) Lookup(ip string) Record {
	var rec dbipRecord
	if err := s.reader.Lookup(net.ParseIP(ip), &rec); err != nil {
		return Record{}
	}
	province := ""
	if len(rec.Subdivisions) > 0 {
		province = rec.Subdivisions[0].Names["en"]
	}
	return Record{
		CountryISO: rec.Country.ISOCode,
		Province:   province,
		City:       rec.City.Names["en"],
	}
}

// ── DB-IP ASN 源(全球 IP → ASN + 组织名,判机房)────────────────────────────
// 与归属地正交:只填 Record 的 ASN/ASNOrg/IsHosting,不碰 CountryISO/Province/City/ISP。
type dbipASNSource struct {
	reader  *maxminddb.Reader // mmap,并发安全
	tmpPath string            // 非空 = 来自内嵌库解压的临时文件,Close 时一并删;外挂文件为空
}

// dbipASNRecord DB-IP ASN Lite / GeoLite2-ASN 兼容 schema。
type dbipASNRecord struct {
	ASN    uint   `maxminddb:"autonomous_system_number"`
	ASNOrg string `maxminddb:"autonomous_system_organization"`
}

// NewDBIPASNSource 打开 DB-IP ASN mmdb(外挂文件,直接 mmap)。路径空/打开失败 → 空源。
func NewDBIPASNSource(path string) Source {
	if path == "" {
		logger.Infof("geoip: 未配 DB-IP ASN(路径空),ASN 源禁用")
		return nopSource{}
	}
	reader, err := maxminddb.Open(path)
	if err != nil {
		logger.Errorf("geoip: DB-IP ASN 打开失败 %s,ASN 源禁用: %+v", path, err)
		return nopSource{}
	}
	logger.Infof("geoip: DB-IP ASN 已加载 %s (type=%s)", path, reader.Metadata.DatabaseType)
	return &dbipASNSource{reader: reader}
}

// EmbeddedDBIPASNSource 用**编译期内嵌**的 DB-IP ASN Lite(见 embeddedDBIPASNGz)构造 ASN 源,零配置开箱即用。
// 同 EmbeddedDBIPSource:内存 gunzip → 写临时文件 → maxminddb.Open 走 mmap;实现 io.Closer(Close 关 reader 并删临时文件)。
// 任何失败(解压/落盘/打开)→ 空源(降级,不 panic)。
func EmbeddedDBIPASNSource() Source {
	gz, err := gzip.NewReader(bytes.NewReader(embeddedDBIPASNGz))
	if err != nil {
		logger.Errorf("geoip: 内嵌 DB-IP ASN 解压器创建失败,ASN 源禁用: %+v", err)
		return nopSource{}
	}
	defer gz.Close()

	tmp, err := os.CreateTemp("", "geoip-asn-*.mmdb")
	if err != nil {
		logger.Errorf("geoip: 内嵌 DB-IP ASN 临时文件创建失败,ASN 源禁用: %+v", err)
		return nopSource{}
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, gz); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP ASN 解压落盘失败,ASN 源禁用: %+v", err)
		return nopSource{}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP ASN 临时文件关闭失败,ASN 源禁用: %+v", err)
		return nopSource{}
	}
	reader, err := maxminddb.Open(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		logger.Errorf("geoip: 内嵌 DB-IP ASN 打开失败,ASN 源禁用: %+v", err)
		return nopSource{}
	}
	logger.Infof("geoip: 内嵌 DB-IP ASN 已解压到临时文件并 mmap %s (type=%s)", tmpPath, reader.Metadata.DatabaseType)
	return &dbipASNSource{reader: reader, tmpPath: tmpPath}
}

// resolveASNSource 决定 ASN 源:GEOIP_ASN_PATH 有值 → 外挂文件覆盖;否则 → 内嵌 gz。
func resolveASNSource(path string) Source {
	if path != "" {
		logger.Infof("geoip: GEOIP_ASN_PATH 已配,ASN 源用外挂文件覆盖内嵌库: %s", path)
		return NewDBIPASNSource(path)
	}
	return EmbeddedDBIPASNSource()
}

func (dbipASNSource) Name() string { return "dbip-asn" }

// Close 释放底层 mmap reader;内嵌来源(tmpPath 非空)再删解压出的临时文件。
func (s *dbipASNSource) Close() error {
	err := s.reader.Close()
	if s.tmpPath != "" {
		if rmErr := os.Remove(s.tmpPath); rmErr != nil && err == nil {
			err = rmErr
		}
	}
	return err
}

// Lookup 查 IP 的 ASN + 组织名,并据组织名启发式判 IsHosting。查不到 → 零值 Record。
func (s *dbipASNSource) Lookup(ip string) Record {
	var rec dbipASNRecord
	if err := s.reader.Lookup(net.ParseIP(ip), &rec); err != nil {
		return Record{}
	}
	return Record{ASN: rec.ASN, ASNOrg: rec.ASNOrg, IsHosting: looksLikeHosting(rec.ASNOrg)}
}

// hostingOrgKeywords 是判定「机房/云/托管/Tor 出口」的组织名关键词(小写子串匹配)。
// 覆盖主流云厂商 + 通用托管词 + 已知 Tor 出口运营商用词。**刻意避开** backbone/telecom/mobile/broadband
// 等骨干/终端运营商词,减少把住宅/移动宽带误判成机房。启发式、非权威,按需增删即可。
var hostingOrgKeywords = []string{
	// 通用托管/机房词
	"hosting", "host", "cloud", "data center", "datacenter", "server", "vps", "dedicated",
	"colo", "colocation", "virtual", "infrastructure", "networks solutions", "internet solutions",
	// 主流云 / 托管厂商
	"amazon", "aws", "azure", "microsoft", "google", "digitalocean", "digital ocean", "ovh",
	"hetzner", "linode", "vultr", "leaseweb", "choopa", "contabo", "scaleway", "oracle",
	"alibaba", "aliyun", "tencent", "huawei cloud", "ucloud", "m247", "datacamp", "g-core",
	"gcore", "fastly", "akamai", "psychz", "quadranet", "hostwinds", "namecheap", "gigenet",
	// 匿名/代理/Tor 相关用词(DFRI 等 Tor 出口运营商)
	"vpn", "proxy", "tor ", "digitala fri", "frikt",
}

// looksLikeHosting 组织名(转小写)命中任一关键词即判为机房/托管类网络。org 空 → false。
func looksLikeHosting(org string) bool {
	if org == "" {
		return false
	}
	l := strings.ToLower(org)
	for _, kw := range hostingOrgKeywords {
		if strings.Contains(l, kw) {
			return true
		}
	}
	return false
}

// ── aurora Feature 封装 ───────────────────────────────────────────────────────

// Config 从环境变量读两个本地 IP 库的路径,都可选(两库都已内嵌,不配即用内嵌库):
//   - 国内:GEOIP_IP2REGION_PATH 不配 → 用**内嵌**库;配了 → 用该外挂文件覆盖(便于线下更新)。
//   - 国外:GEOIP_DBIP_PATH 不配 → 用**内嵌**库;配了 → 用该外挂 DB-IP City mmdb 覆盖。
type Config struct {
	IP2RegionPath string `env:"GEOIP_IP2REGION_PATH,omitempty"` // 国内 ip2region_v4.xdb 路径(空=用内嵌库)
	DBIPPath      string `env:"GEOIP_DBIP_PATH,omitempty"`      // 国外 DB-IP City mmdb 路径(空=用内嵌库)
	ASNPath       string `env:"GEOIP_ASN_PATH,omitempty"`       // ASN DB-IP ASN mmdb 路径(空=用内嵌库);仅在 WithASNEnabled(true) 时才加载
}

// geoipFeature 实现 contracts.Features,把 Resolver 注入 DI 容器。
type geoipFeature struct {
	cfg     Config
	opts    []Option
	closers []io.Closer // 需要在 Close 时释放的源(国外 mmap reader / 内嵌那份还要删临时文件);国内内存 buffer 无需关
}

// NewFeature 构造 geoip feature。opts 为收口库选项(如 WithChinaFallback(true)),
// 两个源的路径从环境变量读(见 Config)。用法:
//
//	app.AddFeature(geoip.NewFeature(geoip.WithChinaFallback(true)))
func NewFeature(opts ...Option) contracts.Features {
	var cfg Config
	if err := config.ResolveConfig(&cfg); err != nil {
		log.Fatalf("geoip: failed to load config: %v", err)
	}
	return &geoipFeature{cfg: cfg, opts: opts}
}

func (f *geoipFeature) Name() string { return "geoip" }

// Setup 用配置路径构造两个源 + 传入的 opts 建 Resolver,并注入容器。
// 路径空/文件缺失 → 该源降级为空,不 panic、不阻断。
func (f *geoipFeature) Setup(app contracts.App) error {
	cn := resolveChinaSource(f.cfg.IP2RegionPath) // env 有值→外挂覆盖;否则→内嵌 gz(开箱即用)
	intl := resolveIntlSource(f.cfg.DBIPPath)     // 同上;内嵌那份解压到临时文件后 mmap
	sources := []Source{cn, intl}

	// ASN 源**默认不装**:仅当调用方传了 WithASNEnabled(true) 才按 env/内嵌加载(不写死)。
	// 先把 opts 应用到一个探针 config 读出这个开关(opts 只在 New 里生效,Feature 需提前知道要不要建源)。
	var probe resolverConfig
	for _, o := range f.opts {
		if o != nil {
			o(&probe)
		}
	}
	// 基础路由选项在前,调用方 opts(如 WithChinaFallback)在后,可覆盖/追加。
	opts := make([]Option, 0, len(f.opts)+3)
	opts = append(opts, WithChina(cn), WithInternational(intl))
	if probe.asnEnabled {
		asn := resolveASNSource(f.cfg.ASNPath) // env 有值→外挂覆盖;否则→内嵌 gz
		sources = append(sources, asn)
		opts = append(opts, WithASN(asn))
	}
	opts = append(opts, f.opts...)

	for _, s := range sources { // 凡实现 Closer 的源(外挂/内嵌 mmap reader)都登记,Close 时释放
		if c, ok := s.(io.Closer); ok {
			f.closers = append(f.closers, c)
		}
	}

	resolver := New(opts...)
	app.ProvideAs(resolver, (*Resolver)(nil))
	return nil
}

// Close 释放已登记的源:国外 mmap reader(内嵌那份还会删掉解压出的临时文件,不泄漏)。
// 国内 ip2region 是内存 buffer,由 GC 回收,不在此列。返回第一个出错的 Close(其余仍会尝试)。
func (f *geoipFeature) Close() error {
	var firstErr error
	for _, c := range f.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
