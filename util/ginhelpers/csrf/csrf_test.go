package csrf_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/can3p/gogo/util/ginhelpers/csrf"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCSRFMiddleware(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		formField  string
		wantStatus int
	}{
		{"no token", "", "", http.StatusForbidden},
		{"wrong header", "wrong", "", http.StatusForbidden},
		{"wrong form field", "", "wrong", http.StatusForbidden},
		{"right header", "session-token", "", http.StatusOK},
		{"right form field", "", "session-token", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			r := gin.New()
			r.POST("/", csrf.CSRFMiddleware(func(*gin.Context) string { return "session-token" }),
				func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"header_csrf": {tc.formField}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.header != "" {
				req.Header.Set("X-CSRFToken", tc.header)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
		})
	}
}
