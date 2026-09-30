// Package csrf rejects state-changing requests that don't carry the session's
// CSRF token, in the X-CSRFToken header or the header_csrf form field.
package csrf

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CSRFMiddleware(getUserCSRFToken func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		CheckCSRF(c, getUserCSRFToken)
	}
}

func CheckCSRF(c *gin.Context, getUserCSRFToken func(*gin.Context) string) {
	userCSRFToken := getUserCSRFToken(c)
	csrfToken := c.GetHeader("X-CSRFToken")

	if csrfToken == "" {
		if val, ok := c.GetPostForm("header_csrf"); ok {
			csrfToken = val
		}
	}

	if csrfToken == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if userCSRFToken == "" {
		// abnormal situation, should never happen
		panic("session does not contain csrf token")
	}

	if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(userCSRFToken)) != 1 {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.Next()
}
