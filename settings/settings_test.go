package settings_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/can3p/gogo/settings"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
)

type mailConfig struct {
	Key    string          `long:"key" env:"KEY" required:"true"`
	Secret settings.Secret `long:"secret" env:"SECRET" required:"true"`
}

type serveCmd struct {
	Port string `long:"port" env:"PORT" required:"true"`
	ran  bool
}

func (c *serveCmd) Execute([]string) error {
	c.ran = true
	return nil
}

type seedCmd struct {
	Users int `long:"users" env:"SEED_USERS" required:"true"`
}

func (c *seedCmd) Execute([]string) error { return nil }

type appConfig struct {
	DatabaseURL string     `long:"database-url" env:"DATABASE_URL" required:"true"`
	Mail        mailConfig `group:"mail" namespace:"mj" env-namespace:"MJ"`
	Serve       serveCmd   `command:"serve"`
	Seed        seedCmd    `command:"seed"`
}

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		args    []string
		wantErr string
	}{
		{
			name: "everything set",
			env:  map[string]string{"DATABASE_URL": "postgres://", "MJ_KEY": "k", "MJ_SECRET": "s", "PORT": "8080"},
			args: []string{"serve"},
		},
		{
			name: "flags work as well as the environment",
			env:  map[string]string{"MJ_KEY": "k", "MJ_SECRET": "s"},
			args: []string{"--database-url", "postgres://", "serve", "--port", "8080"},
		},
		{
			name:    "an empty variable is missing",
			env:     map[string]string{"DATABASE_URL": "", "MJ_KEY": "k", "MJ_SECRET": "s", "PORT": "8080"},
			args:    []string{"serve"},
			wantErr: "required settings are missing or empty: --database-url or $DATABASE_URL",
		},
		{
			name: "every missing setting is named, grouped ones with their namespace, " +
				"and the inactive command's are not",
			env:     map[string]string{"MJ_KEY": "k"},
			args:    []string{"serve"},
			wantErr: "required settings are missing or empty: --database-url or $DATABASE_URL, --mj.secret or $MJ_SECRET, --port or $PORT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"DATABASE_URL", "MJ_KEY", "MJ_SECRET", "PORT", "SEED_USERS"} {
				t.Setenv(key, "")
				require.NoError(t, os.Unsetenv(key))
			}
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			var cfg appConfig
			_, err := settings.Parse(flags.NewParser(&cfg, flags.HelpFlag), tc.args)

			if tc.wantErr == "" {
				require.NoError(t, err)
				require.True(t, cfg.Serve.ran)
				return
			}

			require.EqualError(t, err, tc.wantErr)
			require.False(t, cfg.Serve.ran, "the command must not run with missing settings")

			var flagsErr *flags.Error
			require.ErrorAs(t, err, &flagsErr)
			require.Equal(t, flags.ErrRequired, flagsErr.Type)
		})
	}
}

func TestParse_KeepsCommandHandler(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://")
	t.Setenv("MJ_KEY", "k")
	t.Setenv("MJ_SECRET", "s")
	t.Setenv("PORT", "8080")

	var cfg appConfig
	p := flags.NewParser(&cfg, flags.HelpFlag)

	var handled flags.Commander
	p.CommandHandler = func(cmd flags.Commander, _ []string) error {
		handled = cmd
		return nil
	}

	_, err := settings.Parse(p, []string{"serve"})

	require.NoError(t, err)
	require.Same(t, &cfg.Serve, handled)
	require.False(t, cfg.Serve.ran, "the parser's own handler decides whether to run the command")
}

func TestSecret_DoesNotPrint(t *testing.T) {
	cfg := struct {
		Token settings.Secret `json:"token"`
	}{Token: "hunter2"}

	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("config", "token", cfg.Token)

	jsonBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	for _, printed := range []string{
		fmt.Sprint(cfg.Token),
		fmt.Sprintf("%v %+v %#v", cfg, cfg, cfg),
		string(jsonBytes),
		logged.String(),
	} {
		require.NotContains(t, printed, "hunter2")
		require.Contains(t, printed, "[redacted]")
	}

	require.Equal(t, "hunter2", cfg.Token.Reveal())
	require.Empty(t, settings.Secret("").String(), "an unset secret shows as unset")
}
