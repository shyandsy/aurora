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
// 作为 aurora Feature 使用见 NewFeature / Config(读 GEOIP_IP2REGION_PATH、GEOIP_DBIP_PATH 两个环境变量)。
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
	cnFallback    bool
}

// WithChina 指定管**中国大陆**的源(通常 ip2region)。它也负责判定 IP 是否国内(CountryISO=="CN")。
func WithChina(s Source) Option { return func(c *resolverConfig) { c.china = s } }

// WithInternational 指定管**国外**的源(通常 DB-IP)。
func WithInternational(s Source) Option { return func(c *resolverConfig) { c.international = s } }

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
	return &router{cn: c.china, intl: c.international, cnFallback: c.cnFallback}
}

type router struct {
	cn         Source
	intl       Source
	cnFallback bool
}

// Lookup 见 Resolver。非法/空 IP → 空 Record。
func (r *router) Lookup(ip string) Record {
	ip = strings.TrimSpace(ip)
	if ip == "" || net.ParseIP(ip) == nil {
		return Record{}
	}
	if rec := r.cn.Lookup(ip); rec.IsChina() {
		// 国内 → 国内源(中文省/市 + 运营商)。开了兜底且「有省无市」时,用国外库补一个参考市:
		// 仅当国外库的省(转中文后)与 ip2region 的省一致才采信(挡掉 DB-IP 跨省乱标),标 CityApprox。
		if r.cnFallback && rec.City == "" && rec.Province != "" {
			if intl := r.intl.Lookup(ip); intl.City != "" && provinceCN(intl.Province) == rec.Province {
				rec.City = cityCN(intl.City)
				rec.CityApprox = true
			}
		}
		return rec
	}
	return r.intl.Lookup(ip) // 国外(或国内源没命中)→ 国外源
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

// ── aurora Feature 封装 ───────────────────────────────────────────────────────

// Config 从环境变量读两个本地 IP 库的路径,都可选(两库都已内嵌,不配即用内嵌库):
//   - 国内:GEOIP_IP2REGION_PATH 不配 → 用**内嵌**库;配了 → 用该外挂文件覆盖(便于线下更新)。
//   - 国外:GEOIP_DBIP_PATH 不配 → 用**内嵌**库;配了 → 用该外挂 DB-IP City mmdb 覆盖。
type Config struct {
	IP2RegionPath string `env:"GEOIP_IP2REGION_PATH,omitempty"` // 国内 ip2region_v4.xdb 路径(空=用内嵌库)
	DBIPPath      string `env:"GEOIP_DBIP_PATH,omitempty"`      // 国外 DB-IP City mmdb 路径(空=用内嵌库)
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
	for _, s := range []Source{cn, intl} {        // 两条路径(外挂/内嵌)凡实现 Closer 的都登记,Close 时释放
		if c, ok := s.(io.Closer); ok {
			f.closers = append(f.closers, c)
		}
	}

	// 基础路由选项在前,调用方 opts(如 WithChinaFallback)在后,可覆盖/追加。
	opts := make([]Option, 0, len(f.opts)+2)
	opts = append(opts, WithChina(cn), WithInternational(intl))
	opts = append(opts, f.opts...)

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
