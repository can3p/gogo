package forms

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/can3p/gogo/util/transact"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

var ErrValidationFailed = errors.Errorf("Form validation failed")

type FormSaveAction func(*gin.Context, Form)

func FormSaveDefault(keepValues bool) FormSaveAction {
	return func(c *gin.Context, f Form) {
		if !keepValues {
			f.ClearInput()
		}

		f.RenderForm(c)
	}
}

func FormSaveFullReload(c *gin.Context, f Form) {
	//auth.AddFlash(c, f.FormName(), f.FormName())

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
}

func FormSaveRedirect(url string) FormSaveAction {
	return func(c *gin.Context, f Form) {
		//auth.AddFlash(c, f.FormName(), f.FormName())

		c.Header("HX-Redirect", url)
		c.Status(http.StatusOK)
	}
}

type Form interface {
	FormName() string
	ClearInput()
	ShouldBind(c *gin.Context) error
	RenderForm(c *gin.Context)
	SetFormError(message string)
	AddError(fieldName string, message string)
	Save(c context.Context, exec boil.ContextExecutor) (FormSaveAction, error)
	AddTemplateData(field string, value any)
	TemplateData() map[string]interface{}

	// the only thing to implement in the child form
	Validate(c *gin.Context, exec boil.ContextExecutor) error
}

type FormErrors map[string]string

func (fe FormErrors) HasError(fieldName string) bool {
	if fe == nil {
		return false
	}

	_, ok := fe[fieldName]

	return ok
}

func (fe FormErrors) PassedValidation() error {
	if len(fe) == 0 {
		return nil
	}

	return ErrValidationFailed
}

type FormBase[T any] struct {
	Name                 string
	FormTemplate         string
	FormSaved            bool
	KeepValuesAfterSave  bool
	FullPageReloadOnSave bool

	// we make no effort to sync the keys names between
	// errors and struct, we just assume it to be matching.
	// Same goes for the templates - it's completely left
	// to the developer
	Errors            FormErrors
	FormError         string
	Input             *T
	ExtraTemplateData map[string]interface{}
}

func (f *FormBase[T]) FormName() string {
	return f.Name
}

func (f *FormBase[T]) ClearInput() {
	// reset input values to get a pristine form
	f.Input = new(T)
}

func (f *FormBase[T]) ShouldBind(c *gin.Context) error {
	err := c.ShouldBind(&f.Input)

	if err != nil {
		return errors.Wrapf(err, "failed to bind input to the form [%s]", f.Name)
	}

	return nil
}

func (f *FormBase[T]) AddTemplateData(field string, value any) {
	f.ExtraTemplateData[field] = value
}

func (f *FormBase[T]) TemplateData() map[string]interface{} {
	data := map[string]interface{}{
		"Input":     f.Input,
		"Errors":    f.Errors,
		"FormError": f.FormError,
		"FormSaved": f.FormSaved,
	}

	for k, v := range f.ExtraTemplateData {
		data[k] = v
	}

	return data
}

func (f *FormBase[T]) AddError(fieldName string, message string) {
	if f.Errors == nil {
		f.Errors = FormErrors{}
	}

	f.Errors[fieldName] = message
}

func (f *FormBase[T]) SetFormError(message string) {
	f.FormError = message
}

func (f *FormBase[T]) RenderForm(c *gin.Context) {
	if f.FormSaved && f.FullPageReloadOnSave {
		// TODO: bring in dependency
		//auth.AddFlash(c, f.Name, f.Name)

		c.Header("HX-Refresh", "true")
		c.Status(http.StatusOK)
		return
	}

	c.HTML(http.StatusOK, f.FormTemplate, f.TemplateData())
}

func (f *FormBase[T]) Save(c context.Context, exec boil.ContextExecutor) (FormSaveAction, error) {
	f.FormSaved = true

	if f.FullPageReloadOnSave {
		return FormSaveFullReload, nil
	}

	return FormSaveDefault(f.KeepValuesAfterSave), nil
}

func DefaultHandler(c *gin.Context, db *sqlx.DB, form Form) {
	if err := form.ShouldBind(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"explanation": "Failed to process request", "err": err.Error()})
		return
	}

	if err := form.Validate(c, db); err != nil {
		if err != ErrValidationFailed {
			form.SetFormError(err.Error())
		}

		form.RenderForm(c)
		return
	}

	var action FormSaveAction

	if err := transact.Transact(db, func(tx *sql.Tx) error {
		var err error
		action, err = form.Save(c, tx)

		return err
	}); err != nil {
		slog.Error("Failed to save form", "name", form.FormName(), "err", err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	if action == nil {
		action = FormSaveDefault(false)
	}

	action(c, form)
}
