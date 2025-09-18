package csrf

import (
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

	if csrfToken != userCSRFToken {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.Next()
}
