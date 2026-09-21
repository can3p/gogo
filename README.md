# Save forms like your father did

**WARN**: Work in progress, incompatible changes or force push can happen any time

## Packages

### Forms

Forms expect `htmx` to be set up for the application. The principle there
is that we want to keep things simple, adding a form to a website should
be as trivial as it can be. The simplest thing is to avoid touching
the javascript in the first place, right?

With this approach you get custom validation, full control over templates
and an SPA-like behavior without related headaches at the same time.

## 1. Define a Form

```
package forms

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type SettingsGeneralFormInput struct {
	Timezone string `form:"timezone"`
}

type SettingsGeneralForm struct {
	*FormBase[SettingsGeneralFormInput]
	User *core.User
}

func SettingsGeneralFormNew(u *core.User) Form {
	var form Form = &SettingsGeneralForm{
		FormBase: &FormBase[SettingsGeneralFormInput]{
			Name:         "settings_general",
			FormTemplate: "form--settings-general.html",
			Input:        &SettingsGeneralFormInput{},
			ExtraTemplateData: map[string]interface{}{
				"User": u,
			},
		},
		User: u,
	}

	return form
}

func (f *SettingsGeneralForm) Validate(c *gin.Context, boil.ContextExecutor) error {
	if f.Input.Timezone == "" {
		f.AddError("timezone", "timezone is required")
		return ErrValidationFailed
	}

	return nil
}

func (f *SettingsGeneralForm) Save(c context.Context, exec boil.ContextExecutor) (FormSaveAction, error) {
	f.User.Timezone = f.Input.Timezone

	if _, err := f.User.Update(c, exec, boil.Whitelist(
		core.UserColumns.Timezone,
		core.UserColumns.UpdatedAt,
	)); err != nil {
		return nil, errors.Wrapf(err, "failed to save to the db")
	}

	return f.FormBase.Save(FormSaveDefault(true))
}
```

## 2. Define gin handlers

```
	controls.GET("/settings", func(c *gin.Context) {
		userData := auth.GetUserData(c)

    // gather your data there
		c.HTML(http.StatusOK, "settings.html", web.Settings(c, db, &userData))
	})

	controls.POST("/form/save_settings", func(c *gin.Context) {
		userData := auth.GetUserData(c)
		dbUser := userData.DBUser

		form := forms.SettingsGeneralFormNew(dbUser)

		forms.DefaultHandler(c, db, form)
	})
```

### 3. Define templates

The trick is to have a form template as a partial and include it from the page.
All the helpers in the template below are not included into the package. Roll
your own!

#### `settings.html`

```
{{ template "header.html" . }}

{{ $user := .User.DBUser }}
<div class="uk-container">
  <h1 class="uk-heading-medium">Settings</h1>

  <div class="uk-flex-center uk-grid">
    <div class="uk-card uk-card-default uk-card-body uk-width-2-3@m">
      <h3 class="uk-card-title">General</h3>
      {{ template "form--settings-general.html" toMap "User" $user }}
    </div>
  </div>
</div>

{{ template "footer.html" . }}
```

#### `form--settings-general.html`

```
{{ if .FormSaved }}
  {{ template "partial--success-message.html" toMap "Message" "Settings have been saved" }}
{{ end }}

<form class="uk-form-stacked" method="POST"
  action="{{ link "form_save_settings" }}"
  hx-post="{{ link "form_save_settings" }}"
  hx-swap="outerHTML"
  >

  <div class="uk-margin">
    <label class="uk-form-label" for="form-stacked-text">Submit</label>
    <div class="uk-form-controls">
      {{ if and .Errors (ne .Errors.timezone "") }}
      <div class="uk-text-meta uk-text-danger">{{ .Errors.timezone }}</div>
      {{ end }}
      <select class="uk-select" name="timezone">
        {{ $selected_tz := .User.Timezone }}
        {{ if (and .Input .Input.Timezone) }}
          {{ $selected_tz = .Input.Timezone }}
        {{ end }}

        {{ range tzlist }}
          <option value="{{ . }}" {{ if eq . $selected_tz }}selected{{ end }}>{{ . }}</option>
        {{ end }}
      </select>
    </div>
  </div>

  <div class="uk-margin">
    <button type="submit" class="uk-button uk-button-primary uk-button-large">Save settings</button>
  </div>
</form>
```

### Testcontainers

The `testcontainers/postgres` package gives each integration test a real, migrated, empty PostgreSQL database of its own, using [testcontainers-go](https://golang.testcontainers.org/).

```go
import (
    "os"
    "testing"

    "github.com/can3p/gogo/testcontainers/postgres"
)

func TestMain(m *testing.M) {
    code := m.Run()
    _ = postgres.Cleanup() // optional: stop the container right away
    os.Exit(code)
}

func TestSomething(t *testing.T) {
    t.Parallel()

    db := postgres.New(t, postgres.WithMigrationsDir("../../migrations"))

    // db.DB is a *sqlx.DB, db.SQL a *sql.DB, db.URL the connection string.
    // The database is dropped when the test ends.
}
```

**Features:**
- One container per test binary, shared by all tests
- Migrations are applied once into a template database; each test gets a copy made with `CREATE DATABASE ... TEMPLATE`, so setup cost does not grow with the number of migrations
- Unique database per test, safe with `t.Parallel()`
- The migrations directory is required and checked, so a wrong path fails loudly instead of running tests against an empty schema
- Migrations are recorded in the `migrations` table by default (`WithMigrationsTable` to change), matching what the sql-migrate CLI reads from `dbconfig.yml` rather than the Go API's `gorp_migrations` default
- `WithImage` overrides the default `postgres:16-alpine` image
- Uses the `lib/pq` driver; `URL` is a standard `postgres://` URL, so it can be opened with pgx or passed to a subprocess

`NewTestDB(Options{...})` still works but is deprecated in favour of `New`.
