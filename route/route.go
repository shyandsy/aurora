package route

import (
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
)

type CustomizedHandlerFunc func(*gin.Context) (interface{}, bizerr.BizError)

type Route struct {
	Method      string
	Path        string
	Handler     CustomizedHandlerFunc
	Middlewares []gin.HandlerFunc
}
