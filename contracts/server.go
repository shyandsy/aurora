package contracts

import "github.com/gin-gonic/gin"

// ErrorHandler handles HTTP error responses. Implement this interface to customize error handling.
type ErrorHandler interface {
	HandleError(c *gin.Context, err error)
}

type ServerFeature interface {
	Features
	RegisterRoutes(routes []Route)
	Start() error
	Wait()
}
