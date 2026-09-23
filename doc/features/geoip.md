# GeoIP(IP → 归属地)

把一个 IP **本地离线**解析成归属地(国家 / 省 / 市 / 运营商),单一出口:调用方只依赖 `Resolver.Lookup(ip)`。路由策略(国内用 ip2region、国外用 DB-IP)在构造时定死,调用方无感知。**IP 只查本地库,绝不外发。**

包:`github.com/shyandsy/aurora/feature/geoip`
源码:[geoip.go](../../feature/geoip/geoip.go)(Resolver / Record / 两个源 / Feature 封装)、[city_cn_names.go](../../feature/geoip/city_cn_names.go)(DB-IP 英文省市名 → 中文,给中国兜底用)、[data/](../../feature/geoip/data)(两个库的 gz,`//go:embed` 进二进制)

> 这是一个**自动注册**的 `contracts.Features`(走 `AddFeature`)。**国内库 ip2region 与国外库 DB-IP 都已编译期内嵌**(`//go:embed`),feature 彻底自包含:开箱即用、零配置、**构建/运行都不联网**。env 只在要**外挂覆盖**内嵌库时才用。文件缺失/打不开时对应源降级为空,不 panic、不阻断启动。

---

## 创建 & 用法

```go
// 装配:注册 feature(可传收口库选项,如开启中国兜底)。国内库已内嵌,不配 env 也能解析国内 IP。
app.AddFeature(geoip.NewFeature(geoip.WithChinaFallback(true)))
```

以**接口类型** `geoip.Resolver` 注入,业务里两种取法都行:

```go
// A) 结构体字段注入
type someService struct {
    Geo geoip.Resolver `inject:""`
}

// B) 从容器 Find
var r geoip.Resolver
app.Find(&r)

rec := r.Lookup("112.224.163.187")
// rec.CountryISO == "CN"
// rec.Province   == "山东省"
// rec.ISP        == "联通"
// rec.City / rec.CityApprox 见下
```

`NewFeature(opts ...Option)` 的 `opts` 是收口库选项。目前对外的:

- `WithChinaFallback(on bool)` —— 中国兜底开关(见下)。默认关。

> 两个源的路径不在 `opts` 里(两库都内嵌,env 仅用于外挂覆盖,见[配置](#配置))。`WithChina` / `WithInternational` / `EmbeddedIP2RegionSource` / `EmbeddedDBIPSource` 也是导出的,给「不走 Feature、直接用 `geoip.New(...)` 库」时手动装配用(注意:`EmbeddedDBIPSource()` 返回的源实现 `io.Closer`,用完要 `Close()` 以删掉临时文件)。

**可运行示例**:[`sample/geoip`](../../sample/geoip)。`cd sample/geoip && go run .` —— **不配任何 env** 就打印出国内、国外 IP 的归属地(两库都内嵌,开箱即用)。

### Record 各字段

```go
type Record struct {
    CountryISO string // ISO-3166 alpha-2(CN / US / …);判定国内外的唯一依据
    Province   string // 省 / 州
    City       string // 市
    ISP        string // 运营商(仅国内 ip2region 源有:电信 / 联通 / 移动 …)
    CityApprox bool   // 见下
}
```

未知 / 查不到的字段统一为 `""`。非法 / 空 IP → 空 `Record{}`。

- 国内(`CountryISO == "CN"`):`Province` / `City` 中文,`ISP` 有值。
- 国外:`Province` / `City` 英文,`ISP` 为空。

### 中国兜底与 `CityApprox`(参考市)

国内库 ip2region 有时**只到省、没有市**。开启 `WithChinaFallback(true)` 后,遇到「国内命中但无市」时,会用国外库(DB-IP)兜底补一个市,并把它标成**参考值**:`CityApprox = true`。

采信有闸门:**只有当 DB-IP 的省(转中文后)与 ip2region 的省一致**才补(挡掉 DB-IP 跨省乱标);且 ip2region 已有市时绝不覆盖。DB-IP 对华城市精度有限,所以补来的市统一标 `CityApprox` —— **展示侧应提示「参考」,分析侧应知其非权威**。关闭时(默认)严格只用 ip2region,缺就缺。

---

## 配置

| Env | 类型 | 必填 | 说明 |
|---|---|---|---|
| `GEOIP_IP2REGION_PATH` | string | ❌(omitempty) | 国内库 `ip2region_v4.xdb` 路径。**不配 → 用内嵌库(开箱即用)**;配了 → 用该外挂文件**覆盖**内嵌库(便于线下更新);文件缺失 → 国内源降级为空 |
| `GEOIP_DBIP_PATH` | string | ❌(omitempty) | 国外库 DB-IP City `mmdb` 路径。**不配 → 用内嵌库(开箱即用)**;配了 → 用该外挂文件**覆盖**内嵌库;文件缺失 → 国外源降级为空 |

都可选(`,omitempty`,是加配置的正确姿势,见 [config.md](./config.md))。**零配置**下:两库都用内嵌,国内、国外 IP 都能正常解析。env 仅用于**外挂覆盖**内嵌库(如临时更新某库)。

---

## CI/CD 集成(两库都内嵌,消费方零负担)

**两个库都随二进制内嵌**(`feature/geoip/data/*.gz`,`//go:embed`),消费方**什么都不用做**——不下库、不 gunzip、不 COPY、不设 env,`go build` 出来的二进制就自带完整解析能力,**构建全程不联网**。

**实体 gz 存在版本控制里**(`feature/geoip/data/ip2region_v4.xdb.gz`、`dbip-city-lite.mmdb.gz`),构建/运行都读这两个本地文件,**绝不在 build 时联网下载**——这是有意为之:即便上游数据源将来闭源/下线,已入库的 gz 仍在,feature 照常自包含可用。

唯一的运行时前提:**内嵌 DB-IP 需要一个可写的临时目录**(`os.TempDir()`,容器里通常是 `/tmp`)——它会把内嵌 gz 解压到临时文件再 `mmap`(见[资源与内存](#资源与内存))。绝大多数镜像都有 `/tmp`,无需额外配置;若容器把根文件系统设成只读,给 `/tmp` 挂个可写卷(或用 `TMPDIR` 指到可写目录)即可。

**库来源与许可**(仅在你要**更新内嵌库**时才需要,见下节;日常构建用不到):

| 库 | 用途 | 来源 | 许可 |
|---|---|---|---|
| ip2region `ip2region_v4.xdb` | 国内(中文省市 + 运营商) | `https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb` | **Apache-2.0**,允许再分发(所以能内嵌) |
| DB-IP City Lite `dbip-city-lite.mmdb` | 国外(英文省市) | `https://download.db-ip.com/free/dbip-city-lite-<YYYY-MM>.mmdb.gz` | **CC-BY 4.0**,允许再分发(所以能内嵌);归属:*IP Geolocation by DB-IP*(https://db-ip.com) |

> 选 DB-IP Lite 而非 MaxMind GeoLite2:GeoLite2 的 EULA **禁止再分发**,内嵌/随镜像交付有风险;DB-IP Lite 是 CC-BY,明确允许再分发,才敢内嵌。两者 mmdb schema 一致(`country.iso_code` + `subdivisions[].names` + `city.names`),本 feature 代码通吃。**署名义务**:用了内嵌 DB-IP 的产品,需在合适位置标注 *IP Geolocation by DB-IP*(https://db-ip.com)。

---

## 升级 / 更新 IP 库(将来怎么换新库)

两个库都**内嵌且是数据文件**,schema 长期稳定 → 升级 = **只换 gz、不改代码**。库旧不会崩,只是数据略旧。两库**各自独立更新**,互不影响。**两库都支持 A / B 两种更新方式**:

### A. 换内嵌 gz(改 aurora、发新版)—— 固化给所有消费方

下新库 → `gzip` → 覆盖 `feature/geoip/data/` 下对应 gz → aurora 发版 → 消费方 `make sync-aurora`(或 bump 依赖)拿到新库,一处更新全体生效。

| 库 | 下载 → 覆盖 |
|---|---|
| ip2region | `curl -fsSL https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb \| gzip > feature/geoip/data/ip2region_v4.xdb.gz` |
| DB-IP City Lite | `curl -fsSL "https://download.db-ip.com/free/dbip-city-lite-$(date +%Y-%m).mmdb.gz" -o feature/geoip/data/dbip-city-lite.mmdb.gz`(月初当月若未发布 → 回退上一个月) |

### B. 运行时外挂覆盖(不动 aurora)—— 单服务临时/紧急更新

把新库打进自己镜像,设对应 env 指向它(**优先级高于内嵌**),不等 aurora 发版:

| 库 | env | 说明 |
|---|---|---|
| ip2region | `GEOIP_IP2REGION_PATH=/path/to/ip2region_v4.xdb` | 裸 xdb;整库读进内存 |
| DB-IP | `GEOIP_DBIP_PATH=/path/to/city.mmdb` | 裸 mmdb;直接 mmap(比内嵌那份还省一次解压落盘) |

更新频率:ip2region 不定期(按需拉);DB-IP **每月**发布新库。都是不更新也照常工作,只是数据略旧。许可:ip2region=Apache-2.0;DB-IP=CC-BY 4.0(须署名 *IP Geolocation by DB-IP*, https://db-ip.com)。

### 兼容性

换新库**无需改 `feature/geoip` 代码**:ip2region v4 是「国家\|省\|市\|ISP\|ISO」五段、DB-IP 是 GeoIP2 mmdb schema(`country.iso_code` + `subdivisions[].names` + `city.names`),都长期稳定。只有上游**改了 schema**,才需要动 `geoip.go` 的解析函数。

---

## 隐私

IP **只查进程内的本地库文件**(mmap / 内存 buffer),解析全程离线,**绝不把 IP 发往任何外部服务**。这也是选「本地库随二进制内嵌」而非「调第三方 geo API」的原因。

---

## 资源与内存

两个库的内存/落盘策略不同(库大小差 10 倍):

| 源 | 内存策略 | 临时文件 |
|---|---|---|
| ip2region(国内,~11MB) | 整库解压进内存 `[]byte`,常驻(小,无所谓) | 无 |
| DB-IP(国外,~121MB) | **不常驻**:解压到临时文件 → `maxminddb.Open` 走 **mmap**(冷内存,按需分页,内核可回收) | 有(内嵌那份;外挂路径直接 mmap 用户文件,无临时文件) |

- 内嵌 DB-IP 需要**可写临时目录**(`os.TempDir()`);解压落盘一次(~121MB 磁盘),运行期常驻内存很小(mmap 按需)。外挂(`GEOIP_DBIP_PATH`)直接 mmap 用户提供的文件,连这次落盘都省了。
- **`Feature.Close()`** 会关闭国外源的 mmap reader;**内嵌那份还会 `os.Remove` 掉解压出的临时文件**(不泄漏)。ip2region 内存 buffer 由 GC 回收,无需显式关。App 关闭时正常调用 feature 的 `Close()` 即可。
- 直接用底层库(`EmbeddedDBIPSource()`,不走 Feature)时:返回的源实现 `io.Closer`,**用完必须 `Close()`**,否则临时文件会残留到进程退出。
