// Package ginhelpers answers gin requests with the result of a service call:
// a page, a JSON API response, or the status an apperr error maps to.
package ginhelpers

import (
	"errors"
	"net/http"

	"github.com/can3p/gogo/apperr"
	"github.com/gin-gonic/gin"
	"github.com/samber/mo"
)

// Options configure how failed requests are answered. Set them per router
// with the Configure middleware.
type Options struct {
	// RedirectToLogin answers a page that failed with apperr.ErrNeedsLogin.
	// Without it, the page answers 401.
	RedirectToLogin func(*gin.Context)
	// ShowErrors puts the error text into failed responses. Leave it off in
	// production, where an error can carry internal details.
	ShowErrors bool
}

const optionsKey = "github.com/can3p/gogo/util/ginhelpers.Options"

// Configure sets the options HTML, HTMLError and API use for the requests
// it handles. Without it they use the zero Options: no error text, and 401
// for a page that needs a login.
func Configure(opts Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(optionsKey, opts)
		c.Next()
	}
}

func options(c *gin.Context) Options {
	v, _ := c.Get(optionsKey)
	opts, _ := v.(Options)

	return opts
}

// Status is the HTTP status for an error a page or an API call failed with.
func Status(err error) int {
	var invalid *apperr.ValidationError

	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, apperr.ErrNeedsLogin):
		return http.StatusUnauthorized
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict
	case errors.As(err, &invalid):
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

// HTML renders templateName with the result's value, or answers its error
// with HTMLError.
func HTML[T any](c *gin.Context, templateName string, result mo.Result[T]) {
	if result.IsOk() {
		c.HTML(http.StatusOK, templateName, result.MustGet())
		return
	}

	HTMLError(c, result.Error())
}

// HTMLError answers a page request that failed with err: a redirect to the
// login page for apperr.ErrNeedsLogin when Options.RedirectToLogin is set,
// otherwise Status(err).
func HTMLError(c *gin.Context, err error) {
	opts := options(c)

	if errors.Is(err, apperr.ErrNeedsLogin) && opts.RedirectToLogin != nil {
		opts.RedirectToLogin(c)
		c.Abort()
		return
	}

	httpCode := Status(err)

	if !opts.ShowErrors {
		c.Status(httpCode)
		return
	}

	c.String(httpCode, err.Error())
}

// API answers a JSON API request: {"data": value}, or Status(err) with
// {"errors": [text]}.
func API[T any](c *gin.Context, result mo.Result[T]) {
	if result.IsOk() {
		c.JSON(http.StatusOK, gin.H{
			"data": result.MustGet(),
		})
		return
	}

	httpCode := Status(result.Error())

	if !options(c).ShowErrors {
		c.Status(httpCode)
		return
	}

	c.JSON(httpCode, gin.H{
		"errors": []string{result.Error().Error()},
	})
}
