package forms_test

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/can3p/gogo/forms"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type nameInput struct {
	Name string `form:"name"`
}

type nameForm struct {
	*forms.FormBase[nameInput]
	saveErr error
	saved   string
}

func (f *nameForm) Validate(*gin.Context) error {
	if f.Input.Name == "" {
		f.AddError("name", "name is required")
		return forms.ErrValidationFailed
	}

	return nil
}

func (f *nameForm) Save(c context.Context) (forms.FormSaveAction, error) {
	if f.saveErr != nil {
		return nil, f.saveErr
	}

	f.saved = f.Input.Name

	return f.FormBase.Save(c)
}

func TestDefaultHandler(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		saveErr    error
		wantStatus int
		wantBody   string
		wantSaved  string
	}{
		{"invalid input renders the errors", "", nil, http.StatusOK, "saved=false error=name is required", ""},
		{"valid input is saved", "Ann", nil, http.StatusOK, "saved=true error=", "Ann"},
		{"a failed save is a 500", "Ann", errors.New("db is down"), http.StatusInternalServerError, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			form := &nameForm{
				FormBase: &forms.FormBase[nameInput]{
					Name:         "name",
					FormTemplate: "form.html",
					Input:        &nameInput{},
				},
				saveErr: tc.saveErr,
			}

			r := gin.New()
			r.SetHTMLTemplate(template.Must(template.New("form.html").Parse(`saved={{.FormSaved}} error={{index .Errors "name"}}`)))
			r.POST("/", func(c *gin.Context) { forms.DefaultHandler(c, form) })

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"name": {tc.input}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			require.Equal(t, tc.wantBody, w.Body.String())
			require.Equal(t, tc.wantSaved, form.saved)
		})
	}
}
