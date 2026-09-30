package ginhelpers_test

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/can3p/gogo/apperr"
	"github.com/can3p/gogo/util/ginhelpers"
	"github.com/gin-gonic/gin"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

// serve answers one GET / with handler behind Configure(opts).
func serve(t *testing.T, opts *ginhelpers.Options, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.SetHTMLTemplate(template.Must(template.New("greeting").Parse("hello {{.}}")))
	if opts != nil {
		r.Use(ginhelpers.Configure(*opts))
	}
	r.GET("/", handler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	return w
}

func TestStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{apperr.ErrNotFound, http.StatusNotFound},
		{fmt.Errorf("post 7: %w", apperr.ErrNotFound), http.StatusNotFound},
		{apperr.ErrForbidden, http.StatusForbidden},
		{apperr.ErrNeedsLogin, http.StatusUnauthorized},
		{fmt.Errorf("%w: username is taken", apperr.ErrConflict), http.StatusConflict},
		{fmt.Errorf("saving: %w", apperr.Invalid("title", "Title is required.")), http.StatusBadRequest},
		{errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		require.Equal(t, tc.want, ginhelpers.Status(tc.err), tc.err.Error())
	}
}

func TestHTML(t *testing.T) {
	redirect := func(c *gin.Context) { c.Redirect(http.StatusFound, "/login") }

	cases := []struct {
		name         string
		opts         *ginhelpers.Options
		result       mo.Result[string]
		wantStatus   int
		wantBody     string
		wantLocation string
	}{
		{"a value renders the template", nil, mo.Ok("world"), http.StatusOK, "hello world", ""},
		{"errors are hidden without Configure", nil, mo.Err[string](apperr.ErrForbidden), http.StatusForbidden, "", ""},
		{"errors are shown when asked", &ginhelpers.Options{ShowErrors: true}, mo.Err[string](apperr.ErrNotFound), http.StatusNotFound, "not found", ""},
		{"needs login redirects", &ginhelpers.Options{RedirectToLogin: redirect}, mo.Err[string](apperr.ErrNeedsLogin), http.StatusFound, `<a href="/login">Found</a>.` + "\n\n", "/login"},
		{"needs login without a redirect is 401", nil, mo.Err[string](apperr.ErrNeedsLogin), http.StatusUnauthorized, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(t, tc.opts, func(c *gin.Context) {
				ginhelpers.HTML(c, "greeting", tc.result)
			})

			require.Equal(t, tc.wantStatus, w.Code)
			require.Equal(t, tc.wantBody, w.Body.String())
			require.Equal(t, tc.wantLocation, w.Header().Get("Location"))
		})
	}
}

func TestAPI(t *testing.T) {
	cases := []struct {
		name       string
		opts       *ginhelpers.Options
		result     mo.Result[string]
		wantStatus int
		wantBody   string
	}{
		{"a value is the data", nil, mo.Ok("bar"), http.StatusOK, `{"data":"bar"}`},
		{"errors are hidden without Configure", nil, mo.Err[string](apperr.ErrForbidden), http.StatusForbidden, ""},
		{"errors are shown when asked", &ginhelpers.Options{ShowErrors: true}, mo.Err[string](apperr.ErrConflict), http.StatusConflict, `{"errors":["conflict"]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(t, tc.opts, func(c *gin.Context) {
				ginhelpers.API(c, tc.result)
			})

			require.Equal(t, tc.wantStatus, w.Code)
			require.Equal(t, tc.wantBody, w.Body.String())
		})
	}
}
