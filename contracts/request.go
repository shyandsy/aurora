package contracts

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type RequestContext struct {
	*gin.Context
	App        App
	Translator Translator
}

func (c *RequestContext) GetLang() string {
	if lang := c.Query("lang"); lang != "" {
		return lang
	}

	acceptLang := c.GetHeader("Accept-Language")
	if acceptLang != "" {
		parts := strings.Split(acceptLang, ",")
		if len(parts) > 0 {
			lang := strings.TrimSpace(strings.Split(parts[0], ";")[0])
			return lang
		}
	}

	if c.Translator != nil {
		return c.Translator.GetLang()
	}

	return "en"
}

func (c *RequestContext) T(id string, data ...interface{}) string {
	if c.Translator == nil {
		return id
	}

	lang := c.GetLang()
	return c.Translator.TWithLang(lang, id, data...)
}
