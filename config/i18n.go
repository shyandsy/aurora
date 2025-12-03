package config

type I18NConfig struct {
	DefaultLang    string   `env:"I18N_DEFAULT_LANG,omitempty"`
	SupportedLangs []string `env:"I18N_SUPPORTED_LANGS,omitempty"`
	LocaleDir      string   `env:"I18N_LOCALE_DIR,omitempty"`
	LoadEmbedded   bool     `env:"I18N_LOAD_EMBEDDED,omitempty"`
}

func (s *I18NConfig) Key() string {
	return "i18n"
}

func (s *I18NConfig) Validate() error {
	if s.DefaultLang == "" {
		return NewConfigError("I18N_DEFAULT_LANG is required")
	}

	if len(s.SupportedLangs) == 0 {
		return NewConfigError("I18N_SUPPORTED_LANGS is required")
	}

	defaultLangSupported := false
	for _, lang := range s.SupportedLangs {
		if lang == s.DefaultLang {
			defaultLangSupported = true
			break
		}
	}

	if !defaultLangSupported {
		return NewConfigError("I18N_DEFAULT_LANG must be in I18N_SUPPORTED_LANGS")
	}

	return nil
}
