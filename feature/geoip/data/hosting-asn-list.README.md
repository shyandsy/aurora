# hosting-asn-list.csv

「云 / 托管 / colo / Tor 出口」的 ASN 号清单,是 `IsHosting` 判定的**唯一权威依据**:一个 IP 是不是机房网络,
只看它的 ASN 号在不在这张表里,**不做任何组织名猜测**。按号而不按名的理由——确定、可审计、可维护:
判定 100% 由清单决定(「为什么判它是机房?」→「AS20473 在清单里」),漏判就补录一个号、影响面精确到那一个号,
也不会因为某住宅运营商名字里恰好含 "cloud"/"server" 而被错杀(法人名不含关键词的机房如 AS20473 = Vultr/Choopa
也照样按号抓到)。代价是覆盖范围 = 清单范围:清单没收录的机房 ASN 会判 false,靠定期同步/补录扩覆盖。

- **来源**:[brianhama/bad-asn-list](https://github.com/brianhama/bad-asn-list),**MIT** 许可(可再分发,保留出处)。
- **格式**:两列 CSV `ASN,Entity`(首行表头)。解析只取第一列 ASN 号(见 `../hosting_asn.go` 的 `loadHostingASNSet`)。
- **用法**:`//go:embed data/hosting-asn-list.csv` 编译期内嵌;首次用到 ASN 面时惰性解析成 ASN 号集合。
- **判定收口**:`isHostingASN(asn)` = ASN 号在清单里则 true,否则 false;`asn==0` 一律 false。只此一条,无兜底。

## 刷新(定期同步上游)

```bash
curl -sSL -o feature/geoip/data/hosting-asn-list.csv \
  https://raw.githubusercontent.com/brianhama/bad-asn-list/master/bad-asn-list.csv
```

清单社区维护、非实时权威;偶尔同步即可。补录单个 ASN 也可直接往 CSV 追加一行(`号,备注`)。命中只驱动业务侧的
「软动作」(如注册要求邮件激活),即使个别误收也不至锁死真人。
