package contracts

type Translator interface {
	T(id string, data ...interface{}) string
	TWithLang(lang, id string, data ...interface{}) string
	SetLang(lang string)
	GetLang() string
	SupportedLanguages() []string
}
