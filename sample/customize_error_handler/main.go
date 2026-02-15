package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/app"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature"
)

// CustomErrorHandler implements contracts.ErrorHandler to return custom error JSON.
type CustomErrorHandler struct{}

func (CustomErrorHandler) HandleError(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	msg := err.Error()
	if e, ok := err.(bizerr.BizError); ok {
		code = e.HTTPCode()
		msg = e.Message()
	}
	c.JSON(code, gin.H{
		"code":      code,
		"message":   msg,
		"error":     err.Error(),
		"timestamp": time.Now().Unix(),
		"custom":    true,
	})
}

func main() {
	a := app.NewApp()

	a.AddFeature(feature.NewServerFeature(
		feature.WithErrorHandler(CustomErrorHandler{}),
	))

	a.RegisterRoutes([]contracts.Route{
		{
			Method:  "GET",
			Path:    "/ok",
			Handler: handleOK,
		},
		{
			Method:  "GET",
			Path:    "/err",
			Handler: handleErr,
		},
		{
			Method:  "GET",
			Path:    "/err-bad-request",
			Handler: handleErrBadRequest,
		},
	})

	if err := a.Run(); err != nil {
		log.Fatalf("run failed: %v", err)
	}
}

func handleOK(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	return gin.H{"status": "ok", "message": "success"}, nil
}

func handleErr(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	return nil, bizerr.New(http.StatusInternalServerError, errors.New("intended internal error for test"))
}

func handleErrBadRequest(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	return nil, bizerr.ErrBadRequest(errors.New("bad request for test"))
}
