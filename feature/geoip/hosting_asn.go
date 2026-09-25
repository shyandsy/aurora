package geoip

import (
	"bufio"
	"bytes"
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// embeddedHostingASNCSV 是**编译期内嵌**的「云 / 托管 / colo / Tor 出口」ASN 号清单(两列 CSV:ASN,Entity)。
// 来源 brianhama/bad-asn-list(MIT),见 data/hosting-asn-list.README.md。随二进制交付、构建不联网。
//
// 这张清单是 IsHosting 判定的**唯一权威依据**:一个 IP 是不是「机房/托管网络」,只看它的 ASN 号在不在这张
// 表里,不做任何组织名猜测。为什么按号而不按名:
//   - 确定、可审计:判定结果 100% 由清单决定。「为什么判它是机房?」→「AS20473 在清单第 N 行」,一句话说清。
//   - 可维护:漏判了就把那个 ASN 加进清单,改动影响面精确到一个号,不会像关键词那样牵连一片。
//   - 不误伤:号是精确的,不会因为某住宅运营商名字里恰好含 "cloud"/"server" 而被错杀。
//
// 代价是**覆盖范围 = 清单范围**:清单没收录的机房 ASN 会判 false。这是有意的取舍——宁可少判一个、靠维护清单
// 补齐(可预测),也不要靠猜名字多判一批却说不清为什么(不可预测)。清单社区维护,按 README 定期同步刷新即可。
//
//go:embed data/hosting-asn-list.csv
var embeddedHostingASNCSV []byte

var (
	hostingASNOnce sync.Once
	hostingASNSet  map[uint]struct{}
)

// loadHostingASNSet 惰性解析内嵌清单为「ASN 号集合」(只在首次真正用到 ASN 面时加载,ASN 源没启用则零成本)。
// 每行取第一列(逗号前)为 ASN 号;表头 "ASN" 解析失败自然跳过;去引号 / 去 AS 前缀后按十进制解析。
func loadHostingASNSet() {
	set := make(map[uint]struct{}, 1024)
	sc := bufio.NewScanner(bytes.NewReader(embeddedHostingASNCSV))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		field := line
		if i := strings.IndexByte(line, ','); i >= 0 {
			field = line[:i]
		}
		field = strings.TrimSpace(field)
		field = strings.Trim(field, `"`)
		field = strings.TrimPrefix(strings.ToUpper(field), "AS")
		if n, err := strconv.ParseUint(strings.TrimSpace(field), 10, 32); err == nil && n > 0 {
			set[uint(n)] = struct{}{}
		}
	}
	hostingASNSet = set
}

// isHostingASN 是「是否机房/云/托管/Tor 出口」的**收口判定**(唯一入口,dbipASNSource.Lookup 调它):
// ASN 号在内嵌权威清单里 → true,否则 false。只看号,不看组织名——确定、可审计、可维护(见上方清单说明)。
// asn==0(未知/未启用/私网)一律 false。
func isHostingASN(asn uint) bool {
	if asn == 0 {
		return false
	}
	hostingASNOnce.Do(loadHostingASNSet)
	_, ok := hostingASNSet[asn]
	return ok
}
