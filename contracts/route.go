package contracts

import (
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
)

type CustomizedHandlerFunc func(*RequestContext) (interface{}, bizerr.BizError)

type Route struct {
	Method      string
	Path        string
	Handler     CustomizedHandlerFunc
	Middlewares []gin.HandlerFunc
}
