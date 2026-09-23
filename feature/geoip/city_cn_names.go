package geoip

import "strings"

// 本文件:DB-IP(国外库)英文省/市名 → 中文,用于「ip2region 有省无市」时用国外库补一个**参考市**。
//
// 为什么手维护而非脚本生成:城市名翻译是语言事实(jinan=济南市),不能靠 IP 相关性推断
// —— 实测 DB-IP 的英文市名覆盖范围很粗(一个 "Jinan" 网段横跨济南/青岛/临沂/淄博),
// 按 IP 配对会把 jinan 投成青岛。所以这里只做「英文名 → 该名对应的中文市」的字面翻译,
// 是否采用由调用方(router)用**省级一致**闸门把关(见 geoip.go)。
//
// key 一律**小写**:DB-IP 大小写不保证一致,查询时把英文名转小写再查,匹配更稳(见 cityCN/provinceCN)。
// 覆盖:省会 + 主要地级市 + 实测会落到这条路径的名字(过省级闸门的全量词表)。未收录的英文名 →
// 保持英文(回落),这样将来 DB-IP 更新出现新城市、代码没跟上时也不会崩,只是显示英文(仍标「参考」)。
//
// 已知模糊:DB-IP 用同一个 "Taizhou" 指泰州(江苏)和台州(浙江),按名只能取其一(此处取泰州市)
// —— 正因这类模糊,补来的市才统一标「参考」。

// provinceCNNames:DB-IP 英文省名(小写)→ 中文省名(含「省/市/自治区」等,与 ip2region 口径一致)。
// 用于把 DB-IP 的省和 ip2region 的中文省做一致性比对(省不一致就不采信 DB-IP 的市)。
var provinceCNNames = map[string]string{
	"anhui": "安徽省", "beijing": "北京市", "chongqing": "重庆市", "fujian": "福建省",
	"gansu": "甘肃省", "guangdong": "广东省", "guangxi": "广西", "guizhou": "贵州省",
	"hainan": "海南省", "hebei": "河北省", "heilongjiang": "黑龙江省", "henan": "河南省",
	"hubei": "湖北省", "hunan": "湖南省", "inner mongolia": "内蒙古", "jiangsu": "江苏省",
	"jiangxi": "江西省", "jilin": "吉林省", "liaoning": "辽宁省", "ningxia": "宁夏",
	"qinghai": "青海省", "shaanxi": "陕西省", "shandong": "山东省", "shanghai": "上海市",
	"shanxi": "山西省", "sichuan": "四川省", "tianjin": "天津市", "tibet": "西藏",
	"xinjiang": "新疆", "yunnan": "云南省", "zhejiang": "浙江省",
}

// cityCNNames:DB-IP 英文市名(小写、去括号后缀)→ 中文市名。手工校准,只收有把握的。
var cityCNNames = map[string]string{
	// 直辖市
	"beijing": "北京市", "shanghai": "上海市", "tianjin": "天津市", "chongqing": "重庆市",
	// 华北
	"shijiazhuang": "石家庄市", "tangshan": "唐山市", "zhangjiakou": "张家口市", "chengde": "承德市",
	"baoding": "保定市", "langfang": "廊坊市", "cangzhou": "沧州市", "handan": "邯郸市", "xingtai": "邢台市",
	"taiyuan": "太原市", "datong": "大同市", "yangquan": "阳泉市", "changzhi": "长治市", "linfen": "临汾市", "yuncheng": "运城市",
	"hohhot": "呼和浩特市", "baotou": "包头市", "ordos": "鄂尔多斯市", "chifeng": "赤峰市",
	// 东北
	"shenyang": "沈阳市", "dalian": "大连市", "anshan": "鞍山市", "fushun": "抚顺市", "jinzhou": "锦州市", "dandong": "丹东市", "liaoyang": "辽阳市",
	"changchun": "长春市", "jilin": "吉林市", "siping": "四平市",
	"harbin": "哈尔滨市", "qiqihar": "齐齐哈尔市", "daqing": "大庆市", "mudanjiang": "牡丹江市",
	// 华东
	"nanjing": "南京市", "wuxi": "无锡市", "xuzhou": "徐州市", "changzhou": "常州市", "suzhou": "苏州市",
	"nantong": "南通市", "yancheng": "盐城市", "yangzhou": "扬州市", "zhenjiang": "镇江市", "taizhou": "泰州市", "lianyungang": "连云港市", "huaian": "淮安市",
	"hangzhou": "杭州市", "ningbo": "宁波市", "wenzhou": "温州市", "jiaxing": "嘉兴市", "huzhou": "湖州市",
	"shaoxing": "绍兴市", "jinhua": "金华市", "quzhou": "衢州市", "lishui": "丽水市", "zhoushan": "舟山市", "zhuji": "诸暨市",
	"hefei": "合肥市", "wuhu": "芜湖市", "bengbu": "蚌埠市", "anqing": "安庆市", "fuyang": "阜阳市",
	"fuzhou": "福州市", "xiamen": "厦门市", "quanzhou": "泉州市", "zhangzhou": "漳州市", "putian": "莆田市", "longyan": "龙岩市", "ningde": "宁德市",
	"nanchang": "南昌市", "jiujiang": "九江市", "ganzhou": "赣州市", "shangrao": "上饶市",
	"jinan": "济南市", "qingdao": "青岛市", "zibo": "淄博市", "yantai": "烟台市", "weifang": "潍坊市",
	"jining": "济宁市", "linyi": "临沂市", "weihai": "威海市", "dongying": "东营市", "tai'an": "泰安市", "taian": "泰安市", "zaozhuang": "枣庄市", "dezhou": "德州市", "liaocheng": "聊城市", "binzhou": "滨州市", "heze": "菏泽市", "rizhao": "日照市",
	// 华中
	"zhengzhou": "郑州市", "luoyang": "洛阳市", "kaifeng": "开封市", "xinxiang": "新乡市", "nanyang": "南阳市", "anyang": "安阳市", "xuchang": "许昌市", "zhoukou": "周口市",
	"wuhan": "武汉市", "huangshi": "黄石市", "yichang": "宜昌市", "xiangyang": "襄阳市", "jingzhou": "荆州市", "shiyan": "十堰市", "xiaogan": "孝感市",
	"changsha": "长沙市", "zhuzhou": "株洲市", "xiangtan": "湘潭市", "hengyang": "衡阳市", "yueyang": "岳阳市", "changde": "常德市",
	// 华南
	"guangzhou": "广州市", "shenzhen": "深圳市", "dongguan": "东莞市", "foshan": "佛山市", "zhuhai": "珠海市",
	"zhongshan": "中山市", "huizhou": "惠州市", "jiangmen": "江门市", "zhaoqing": "肇庆市", "shantou": "汕头市",
	"zhanjiang": "湛江市", "maoming": "茂名市", "qingyuan": "清远市", "shaoguan": "韶关市", "jieyang": "揭阳市", "meizhou": "梅州市",
	"nanning": "南宁市", "liuzhou": "柳州市", "guilin": "桂林市", "beihai": "北海市",
	"haikou": "海口市", "sanya": "三亚市", "qionghai": "琼海市", "wenchang": "文昌市",
	// 西南
	"chengdu": "成都市", "mianyang": "绵阳市", "deyang": "德阳市", "nanchong": "南充市", "yibin": "宜宾市", "luzhou": "泸州市", "leshan": "乐山市",
	"guiyang": "贵阳市", "zunyi": "遵义市", "anshun": "安顺市",
	"kunming": "昆明市", "qujing": "曲靖市", "yuxi": "玉溪市", "dali": "大理市",
	"lhasa": "拉萨市", "nyingchi": "林芝市",
	// 西北
	"xi'an": "西安市", "xian": "西安市", "xianyang": "咸阳市", "xianyangshi": "咸阳市", "baoji": "宝鸡市", "weinan": "渭南市", "hanzhong": "汉中市",
	"lanzhou": "兰州市", "tianshui": "天水市",
	"xining":   "西宁市",
	"yinchuan": "银川市", "shizuishan": "石嘴山市",
	"urumqi": "乌鲁木齐市", "karamay": "克拉玛依市", "shihezi": "石河子市",
}

// stripParen 去掉 DB-IP 城市名的括号后缀,如 "Guangzhou (Tianhe Qu)" → "Guangzhou"、
// "Wuxi (Jiangyin Shi)" → "Wuxi"。DB-IP 常把区/商圈塞进括号,去掉后才好对城市名。
func stripParen(s string) string {
	if i := strings.IndexByte(s, '('); i > 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// provinceCN 把 DB-IP 英文省名转中文;未收录返回 ""(视为对不上,调用方不采信)。大小写不敏感。
func provinceCN(enProvince string) string {
	return provinceCNNames[strings.ToLower(strings.TrimSpace(enProvince))]
}

// cityCN 把 DB-IP 英文市名转中文(先去括号后缀、转小写);未收录返回**去括号后的原英文**
// （回落,仍可展示,标「参考」)。大小写不敏感。
func cityCN(enCity string) string {
	c := stripParen(enCity)
	if zh, ok := cityCNNames[strings.ToLower(c)]; ok {
		return zh
	}
	return c
}
