package contracts

import (
	"github.com/gin-gonic/gin"
)

type RequestContext struct {
	*gin.Context
	App App
}
