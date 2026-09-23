# geoip sample

IP → 归属地 本地离线解析。国内库(ip2region)与国外库(DB-IP)**都已编译期内嵌**进 `feature/geoip`,所以**不配任何环境变量**就能跑出国内、国外结果——这就是"开箱即用",构建/运行都不联网。

## 运行

```bash
go run .
```

## 预期输出

```
IP → 归属地(国内 + 国外库都已内嵌,零配置):
  112.224.163.187  country=CN province="山东省" city="济南市" isp="联通" approx=true
  116.7.32.9       country=CN province="广东省" city="深圳市" isp="电信" approx=false
  8.8.8.8          country=US province="California" city="Mountain View" isp="" approx=false
```

- 国内 IP:内嵌 ip2region 给中文省/市 + 运营商。第一条 `112.224.163.187` ip2region 只到省(无市),因内嵌了国外库、又开了 `WithChinaFallback(true)`,用 DB-IP 在**省一致**下补了参考市「济南市」→ `approx=true`(参考值)。
- 国外 IP `8.8.8.8`:内嵌 DB-IP 给英文省/市(`isp` 为空,DB-IP 无运营商)。
- 国外源来自内嵌库,底层是解压到临时文件后 mmap;示例退出时 `defer Close()` 释放并删除临时文件。

> 生产装配用 `app.AddFeature(geoip.NewFeature(...))` + `app.Find(&resolver)`;本示例为零配置演示直接 `geoip.New(...)`,拿到的是同一个 `Resolver`。库更新/外挂覆盖见 [doc/features/geoip.md](../../doc/features/geoip.md)。
