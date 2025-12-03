package feature

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
)

//go:embed i18n/*.yaml
var embeddedFrameworkLocales embed.FS

type I18NService interface {
	contracts.Features
	contracts.Translator
}

type i18nFeature struct {
	Config      *config.I18NConfig
	bundle      *i18n.Bundle
	localizer   *i18n.Localizer
	currentLang string
}

func NewI18NFeature() contracts.Features {
	cfg := &config.I18NConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		log.Fatalf("Failed to load i18n config: %v", err)
	}
	return &i18nFeature{Config: cfg}
}

func (f *i18nFeature) Name() string {
	return "i18n"
}

func (f *i18nFeature) Setup(app contracts.App) error {
	if f.Config.DefaultLang == "" {
		f.Config.DefaultLang = "en"
	}
	if len(f.Config.SupportedLangs) == 0 {
		f.Config.SupportedLangs = []string{"en", "zh-CN"}
	}
	if f.Config.LocaleDir == "" {
		f.Config.LocaleDir = "locales"
	}

	if err := f.Config.Validate(); err != nil {
		return fmt.Errorf("i18n configuration validation failed: %w", err)
	}

	f.bundle = i18n.NewBundle(language.Make(f.Config.DefaultLang))
	f.bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	f.bundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)
	f.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	f.bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load framework locale files (always use embedded mode for framework)
	if err := f.loadEmbeddedFrameworkLocaleFiles(); err != nil {
		log.Printf("Failed to load embedded framework locale files: %v", err)
	}

	if err := f.loadLocaleFiles(); err != nil {
		return fmt.Errorf("failed to load locale files: %w", err)
	}

	f.currentLang = f.Config.DefaultLang
	f.localizer = i18n.NewLocalizer(f.bundle, f.currentLang)

	app.ProvideAs(f, (*I18NService)(nil))
	app.ProvideAs(f, (*contracts.Translator)(nil))

	return nil
}

func (f *i18nFeature) Close() error {
	return nil
}

func (f *i18nFeature) loadEmbeddedFrameworkLocaleFiles() error {
	// Load embedded framework locale files
	for _, lang := range f.Config.SupportedLangs {
		// Try different file formats in priority order: yaml > yml > toml > json
		possibleFiles := []string{
			fmt.Sprintf("i18n/%s.yaml", lang),
			fmt.Sprintf("i18n/%s.yml", lang),
			fmt.Sprintf("i18n/%s.toml", lang),
			fmt.Sprintf("i18n/%s.json", lang),
		}

		loaded := false
		for _, filePath := range possibleFiles {
			data, err := embeddedFrameworkLocales.ReadFile(filePath)
			if err != nil {
				continue
			}

			// Parse the embedded file
			// Use a path format that go-i18n can correctly parse: lang.format
			// For example: zh-CN.yaml instead of i18n/zh-CN.yaml
			langFileName := fmt.Sprintf("%s.%s", lang, "yaml")
			if _, err := f.bundle.ParseMessageFileBytes(data, langFileName); err != nil {
				log.Printf("Failed to parse embedded framework locale file %s: %v", filePath, err)
				continue
			}

			loaded = true
			break
		}

		if !loaded {
			log.Printf("Embedded framework locale file not found for language %s", lang)
		}
	}

	return nil
}

func (f *i18nFeature) loadLocaleFiles() error {
	localeDir := f.Config.LocaleDir

	if !filepath.IsAbs(localeDir) {
		wd, err := os.Getwd()
		if err == nil {
			localeDir = filepath.Join(wd, localeDir)
		}
	}

	if _, err := os.Stat(localeDir); os.IsNotExist(err) {
		log.Printf("Application locale directory not found: %s, using framework translations only", localeDir)
		return nil
	}

	for _, lang := range f.Config.SupportedLangs {
		yamlFile := filepath.Join(localeDir, fmt.Sprintf("%s.yaml", lang))
		ymlFile := filepath.Join(localeDir, fmt.Sprintf("%s.yml", lang))
		tomlFile := filepath.Join(localeDir, fmt.Sprintf("%s.toml", lang))
		jsonFile := filepath.Join(localeDir, fmt.Sprintf("%s.json", lang))

		loaded := false

		if _, err := os.Stat(yamlFile); err == nil {
			if _, err := f.bundle.LoadMessageFile(yamlFile); err != nil {
				log.Printf("Failed to load locale file %s: %v", yamlFile, err)
			} else {
				loaded = true
			}
		} else if _, err := os.Stat(ymlFile); err == nil {
			if _, err := f.bundle.LoadMessageFile(ymlFile); err != nil {
				log.Printf("Failed to load locale file %s: %v", ymlFile, err)
			} else {
				loaded = true
			}
		} else if _, err := os.Stat(tomlFile); err == nil {
			if _, err := f.bundle.LoadMessageFile(tomlFile); err != nil {
				log.Printf("Failed to load locale file %s: %v", tomlFile, err)
			} else {
				loaded = true
			}
		} else if _, err := os.Stat(jsonFile); err == nil {
			if _, err := f.bundle.LoadMessageFile(jsonFile); err != nil {
				log.Printf("Failed to load locale file %s: %v", jsonFile, err)
			} else {
				loaded = true
			}
		}

		if !loaded {
			log.Printf("Locale file not found for language %s (checked %s.yaml, %s.yml, %s.toml, %s.json)", lang, lang, lang, lang, lang)
		}
	}

	return nil
}

func (f *i18nFeature) T(id string, data ...interface{}) string {
	return f.TWithLang(f.currentLang, id, data...)
}

// TWithLang translates message ID using specified language
func (f *i18nFeature) TWithLang(lang, id string, data ...interface{}) string {
	localizer := i18n.NewLocalizer(f.bundle, lang)

	var templateData map[string]interface{}
	if len(data) > 0 {
		if m, ok := data[0].(map[string]interface{}); ok {
			templateData = m
		} else if len(data) == 1 {
			templateData = map[string]interface{}{
				"Value": data[0],
			}
		}
	}

	message, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: templateData,
	})

	if err != nil {
		log.Printf("Translation failed for ID '%s' in language '%s': %v", id, lang, err)
		return id
	}

	return message
}

func (f *i18nFeature) SetLang(lang string) {
	for _, supportedLang := range f.Config.SupportedLangs {
		if supportedLang == lang {
			f.currentLang = lang
			f.localizer = i18n.NewLocalizer(f.bundle, lang)
			return
		}
	}

	log.Printf("Language '%s' is not supported, using default '%s'", lang, f.Config.DefaultLang)
	f.currentLang = f.Config.DefaultLang
	f.localizer = i18n.NewLocalizer(f.bundle, f.Config.DefaultLang)
}

func (f *i18nFeature) GetLang() string {
	return f.currentLang
}

func (f *i18nFeature) SupportedLanguages() []string {
	return f.Config.SupportedLangs
}
