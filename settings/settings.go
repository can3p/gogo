// Package settings holds the helpers an application needs on top of
// github.com/jessevdk/go-flags: every setting is an option with paired
// `long:` and `env:` tags, required settings fail at startup and name their
// environment variable, and credentials are Secrets that don't print.
package settings

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jessevdk/go-flags"
)

// Parse parses args with p and runs the active command, like p.ParseArgs,
// with one difference: a required option whose value is empty counts as
// missing. go-flags alone accepts an environment variable that exists but is
// empty, so a required DATABASE_URL= passes it.
//
// A missing option fails before any command runs, with a *flags.Error of type
// flags.ErrRequired that names every missing option by both its flag and its
// environment variable. Create p without flags.PrintErrors, or go-flags prints
// its own message first.
func Parse(p *flags.Parser, args []string) ([]string, error) {
	next := p.CommandHandler

	defer func() { p.CommandHandler = next }()

	p.CommandHandler = func(cmd flags.Commander, args []string) error {
		if err := checkRequired(p); err != nil {
			return err
		}

		if next != nil {
			return next(cmd, args)
		}

		if cmd != nil {
			return cmd.Execute(args)
		}

		return nil
	}

	rest, err := p.ParseArgs(args)

	// go-flags stops at the first unset required flag it finds, and names only
	// the flag. Name all of them, with their environment variables.
	var flagsErr *flags.Error
	if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrRequired {
		if requiredErr := checkRequired(p); requiredErr != nil {
			return rest, requiredErr
		}
	}

	return rest, err
}

// checkRequired returns an error naming every required option of the parser
// and its active commands whose value is empty.
func checkRequired(p *flags.Parser) error {
	var missing []string

	for cmd := p.Command; cmd != nil; cmd = cmd.Active {
		missing = appendMissing(missing, cmd.Group)
	}

	if len(missing) == 0 {
		return nil
	}

	return &flags.Error{
		Type:    flags.ErrRequired,
		Message: "required settings are missing or empty: " + strings.Join(missing, ", "),
	}
}

func appendMissing(missing []string, g *flags.Group) []string {
	for _, opt := range g.Options() {
		if opt.Required && isEmpty(opt.Value()) {
			missing = append(missing, describe(opt))
		}
	}

	for _, sub := range g.Groups() {
		missing = appendMissing(missing, sub)
	}

	return missing
}

func isEmpty(v any) bool {
	return v == nil || reflect.ValueOf(v).IsZero()
}

// describe names an option the way a user can set it: "--database-url or
// $DATABASE_URL".
func describe(opt *flags.Option) string {
	name := fmt.Sprintf("-%c", opt.ShortName)
	if long := opt.LongNameWithNamespace(); long != "" {
		name = "--" + long
	}

	if env := opt.EnvKeyWithNamespace(); env != "" {
		return name + " or $" + env
	}

	return name
}
