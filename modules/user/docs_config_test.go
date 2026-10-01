package user

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// docs_config_test.go —— 防「文档把 user.Config 字段抄错/抄漏」的回归闸。
//
// 背景:Config 字段此前被抄在 config.go + 模块 README + web README + 设计稿 + 接入 skill 五处,
// 改一处必漏几处——实测模块 README / skill 曾列出根本不存在的 RateLimitNamespace / TablePrefix、
// 漏了 #70 加的 LoginPolicyProvider。靠人自觉全量同步不可靠,故把「文档 == config.go 真实字段」
// 做成跟随 `go test ./...`(CI 已跑)的测试:
//
//   A. 文档里任何 `user.Config{ ... }` 示例中出现的字段键,必须都是 Config 的真实字段(反射为准);
//   B. Config 的每个真实字段,都必须在模块 README 里被提到(保证新增字段不会漏文档)。
//
// 真实字段用**反射**取(不解析 config.go 文本),永不和结构体漂移。

// configDocs 是会出现 user.Config 示例/字段说明、需要被本闸盯住的文档(相对本包目录)。
var configDocs = []string{
	"README.md",
	"web/README.md",
	"../../doc/proposals/shared-user-center.md",
	"../../.claude/skills/user-module-migration/SKILL.md",
}

// realConfigFields 反射出 user.Config 的全部导出字段名(单一真源)。
func realConfigFields() map[string]bool {
	fields := map[string]bool{}
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() {
			fields[f.Name] = true
		}
	}
	return fields
}

// configLiteralRe 抓文档里的 `user.Config{ ... }` 代码字面量(可跨行)。
var configLiteralRe = regexp.MustCompile(`(?s)user\.Config\{(.*?)\}`)

// fieldKeyRe 从字面量体里抓 `FieldName:` 形式的字段键(PascalCase,后跟冒号)。
var fieldKeyRe = regexp.MustCompile(`([A-Z][A-Za-z0-9_]*)\s*:`)

// TestDocs_ConfigLiteralsUseRealFields 闸 A:文档里 user.Config{} 示例的字段键必须都真实存在。
func TestDocs_ConfigLiteralsUseRealFields(t *testing.T) {
	real := realConfigFields()
	for _, doc := range configDocs {
		raw, err := os.ReadFile(doc)
		if err != nil {
			t.Errorf("读文档失败 %s: %v(路径变了?本闸需跟着改)", doc, err)
			continue
		}
		for _, m := range configLiteralRe.FindAllStringSubmatch(string(raw), -1) {
			for _, k := range fieldKeyRe.FindAllStringSubmatch(m[1], -1) {
				key := k[1]
				if !real[key] {
					t.Errorf("%s:user.Config{} 示例用了不存在的字段 %q;真实字段=%v(改文档或 config.go)",
						doc, key, sortedKeys(real))
				}
			}
		}
	}
}

// TestDocs_ModuleReadmeCoversAllConfigFields 闸 B:每个真实 Config 字段都要在模块 README 出现。
func TestDocs_ModuleReadmeCoversAllConfigFields(t *testing.T) {
	real := realConfigFields()
	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("读 modules/user/README.md 失败: %v", err)
	}
	readme := string(raw)
	for f := range real {
		if !strings.Contains(readme, f) {
			t.Errorf("Config 字段 %q 没在 modules/user/README.md 里说明(新增字段漏写文档?)", f)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// 小集合,简单插入排序,稳定输出便于读报错
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
