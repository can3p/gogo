package ginhelpers

import (
	"net/http"

	"github.com/can3p/gogo/util"
	"github.com/friendsofgo/errors"
	"github.com/gin-gonic/gin"
	"github.com/samber/mo"
)

var ErrNotFound = errors.Errorf("not found")
var ErrNeedsLogin = errors.Errorf("needs login")
var ErrForbidden = errors.Errorf("forbidden")
var ErrBadRequest = errors.Errorf("invalid input")

type Redirector interface {
	RedirectToLogin(c *gin.Context)
}

func HTML[T any](c *gin.Context, redirector Redirector, templateName string, result mo.Result[T]) {
	if result.IsOk() {
		c.HTML(http.StatusOK, templateName, result.MustGet())
		return
	}

	var httpCode int = http.StatusInternalServerError

	switch result.Error() {
	case ErrNotFound:
		httpCode = http.StatusNotFound
	case ErrForbidden:
		httpCode = http.StatusForbidden
	case ErrBadRequest:
		httpCode = http.StatusBadRequest
	case ErrNeedsLogin:
		redirector.RedirectToLogin(c)
		c.Abort()
		return
	}

	if util.InCluster() {
		c.Status(httpCode)
		return
	}

	c.String(httpCode, result.Error().Error())
}

func API[T any](c *gin.Context, result mo.Result[T]) {
	if result.IsOk() {
		c.JSON(http.StatusOK, gin.H{
			"data": result.MustGet(),
		})
		return
	}

	var httpCode int = http.StatusInternalServerError

	switch result.Error() {
	case ErrNotFound:
		httpCode = http.StatusNotFound
	case ErrForbidden:
		httpCode = http.StatusForbidden
	case ErrBadRequest:
		httpCode = http.StatusBadRequest
	}

	if util.InCluster() {
		c.Status(httpCode)
		return
	}

	c.JSON(httpCode, gin.H{
		"errors": []string{result.Error().Error()},
	})
}
