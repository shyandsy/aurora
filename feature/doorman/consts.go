package doorman

// Field.Type 的取值:配置页据此渲染控件。集中定义,避免各插件散落魔法字符串。
const (
	FieldString     = "string"
	FieldStringList = "string_list"
	FieldInt        = "int"
	FieldIntList    = "int_list"
	FieldDuration   = "duration"
	FieldSelect     = "select"
)

// 内置条件的类别标识。导出成常量:插件 Type()、业务配规则、配置页下拉都引它,不各写各的裸串。
const (
	TypeUAMatch    = "ua_match"    // 条件:UA 命中关键词
	TypeASNHosting = "asn_hosting" // 条件:IP 属机房/云/Tor
	TypeCountryIn  = "country_in"  // 条件:国家命中集合
)

// 内置条件配置字段的 key。同一个 key 既在 Fields() 声明、又是 Compile 里 JSON 的字段名,提成常量对齐。
const (
	keyPatterns  = "patterns"  // ua_match
	keyCountries = "countries" // country_in
)
