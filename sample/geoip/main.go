// Sample: geoip —— IP → 归属地 本地离线解析,开箱即用。
//
// 国内库 ip2region 与国外库 DB-IP **都已编译期内嵌**进 feature/geoip,所以本示例**不配任何
// 环境变量**就能 `go run .` 把国内、国外 IP 的归属地都打印出来(构建/运行都不联网)。
//
// 说明:aurora 的 App(app.NewApp)在构造时要求一批 Server 相关环境变量(HOST/PORT/…),
// 为了让本示例"零配置即可运行",这里直接用 geoip.New(...) 把 Resolver 建出来演示 Lookup,
// 与走 App 生命周期(AddFeature 后 Find 出 geoip.Resolver)拿到的是同一个东西。生产装配见
// doc/features/geoip.md 的「创建 & 用法」。
//
// 国外源来自内嵌库,底层是解压到临时文件后 mmap;示例退出时 defer Close() 释放并删除临时文件。
package main

import (
	"fmt"

	"github.com/shyandsy/aurora/feature/geoip"
)

func main() {
	// 国内、国外源都用内嵌库(零配置);开启中国兜底(国内有省无市时用国外库补参考市)。
	cn := geoip.EmbeddedIP2RegionSource()
	intl := geoip.EmbeddedDBIPSource()
	if c, ok := intl.(interface{ Close() error }); ok {
		defer c.Close() // 内嵌 DB-IP 走临时文件 mmap,退出前关闭并删除临时文件
	}

	// ASN 面可选:传 WithASN 才查(默认不开)。这里也用内嵌 ASN 库演示。
	asn := geoip.EmbeddedDBIPASNSource()
	if c, ok := asn.(interface{ Close() error }); ok {
		defer c.Close() // 内嵌 ASN 同样走临时文件 mmap,退出前关闭并删除
	}

	r := geoip.New(
		geoip.WithChina(cn),
		geoip.WithInternational(intl),
		geoip.WithChinaFallback(true),
		geoip.WithASN(asn), // 开启 ASN 面:补 ASN / ASNOrg / IsHosting
	)

	ips := []string{
		"112.224.163.187", // 国内:山东 · 联通(住宅,非机房)
		"116.7.32.9",      // 国内:广东
		"8.8.8.8",         // 国外:美国 · Google(机房)
		"171.25.193.36",   // 国外:瑞典 · Tor 出口(DFRI)
	}

	fmt.Println("IP → 归属地 + ASN(全部内嵌,零配置):")
	for _, ip := range ips {
		rec := r.Lookup(ip)
		fmt.Printf("  %-16s country=%-2s province=%q city=%q isp=%q approx=%v | asn=%d org=%q hosting=%v\n",
			ip, rec.CountryISO, rec.Province, rec.City, rec.ISP, rec.CityApprox, rec.ASN, rec.ASNOrg, rec.IsHosting)
	}
}
