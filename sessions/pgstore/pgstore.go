// Package pgstore stores gin sessions (github.com/gin-contrib/sessions, on
// gorilla/sessions) in PostgreSQL.
//
// It reads and writes the rows and cookies of github.com/antonlindstrom/pgstore,
// so an application can switch to it without logging anyone out: the cookie
// holds the session ID encoded with securecookie, and the http_sessions row
// holds the session values encoded with the same codecs.
//
// The store never creates its table. Its migrations ship with the package, in
// sql-migrate format (Migrations), and an application copies them into its own
// migrations directory with InstallMigrations or the pgstore-migrations
// command, then runs them with the rest. Expired rows are not read, but they
// are only deleted by DeleteExpired or Cleanup, which the application starts
// itself.
package pgstore

import (
	"context"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// DefaultMaxAge is the session lifetime of a new Store, in seconds (30 days).
const DefaultMaxAge = 86400 * 30

// Store is a session store on the http_sessions table. It implements both
// gorilla's sessions.Store and gin-contrib's sessions.Store.
type Store struct {
	Codecs  []securecookie.Codec
	options *sessions.Options
	db      *sql.DB
}

var (
	_ sessions.Store    = (*Store)(nil)
	_ ginsessions.Store = (*Store)(nil)
)

// New returns a store on db. keyPairs are securecookie hash and encryption
// key pairs, as for securecookie.CodecsFromPairs. Sessions default to path "/"
// and a max age of DefaultMaxAge.
func New(db *sql.DB, keyPairs ...[]byte) *Store {
	s := &Store{
		Codecs:  securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{Path: "/"},
		db:      db,
	}
	s.MaxAge(DefaultMaxAge)

	return s
}

// Get returns the named session of the request, through the request's
// gorilla session registry.
func (s *Store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New returns the named session of the request without adding it to the
// registry. A missing, undecodable or tampered cookie, and a cookie whose row
// is missing or expired, give a new empty session and no error. Only a failing
// query is an error, returned alongside a new empty session.
func (s *Store) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true

	c, err := r.Cookie(name)
	if err != nil {
		return session, nil
	}

	if err := securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...); err != nil {
		session.ID = ""
		return session, nil
	}

	found, err := s.load(r.Context(), session)
	if err != nil || !found {
		session.ID = ""
		session.Values = map[any]any{}

		return session, err
	}

	session.IsNew = false

	return session, nil
}

// Save writes the session's row and cookie. A session whose Options.MaxAge is
// negative is deleted instead, and its cookie expired.
func (s *Store) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	ctx := r.Context()

	if session.Options.MaxAge < 0 {
		if err := s.destroy(ctx, session.ID); err != nil {
			return err
		}

		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))

		return nil
	}

	if session.ID == "" {
		session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
	}

	if err := s.save(ctx, session); err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.Codecs...)
	if err != nil {
		return fmt.Errorf("failed to encode the session cookie: %w", err)
	}

	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))

	return nil
}

// Options sets the options of new sessions from gin-contrib's options, and
// the codecs' max age to options.MaxAge.
func (s *Store) Options(options ginsessions.Options) {
	s.options = options.ToGorillaOptions()
	s.MaxAge(options.MaxAge)
}

// MaxAge sets the max age of new sessions and of the codecs, in seconds. The
// codecs reject a cookie older than that; 0 turns that check off. A session
// row expires MaxAge seconds after its last save, or DefaultMaxAge after it
// when MaxAge is 0.
func (s *Store) MaxAge(age int) {
	s.options.MaxAge = age

	for _, c := range s.Codecs {
		if sc, ok := c.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

// MaxLength limits the length of an encoded session to l bytes; 0 means no
// limit. securecookie's default is 4096.
func (s *Store) MaxLength(l int) {
	for _, c := range s.Codecs {
		if sc, ok := c.(*securecookie.SecureCookie); ok {
			sc.MaxLength(l)
		}
	}
}

// Regenerate starts a new session in place of s, as login and logout must
// (session fixation): the old session's row is deleted, its values are
// cleared and the next Save writes the session under a fresh ID and cookie.
func Regenerate(ctx context.Context, s ginsessions.Session) error {
	gb, ok := s.(interface{ Session() *sessions.Session })
	if !ok {
		return fmt.Errorf("session %T is not backed by gorilla/sessions", s)
	}

	gs := gb.Session()
	if gs == nil {
		return errors.New("no session to regenerate")
	}

	// gin-contrib's sessions are made by the store itself, so Store() is
	// the *Store handed to ginsessions.Sessions. Regenerate has no request,
	// hence no request context.
	if ps, ok := gs.Store().(*Store); ok && gs.ID != "" {
		if err := ps.destroy(ctx, gs.ID); err != nil {
			return fmt.Errorf("failed to delete the old session: %w", err)
		}
	}

	s.Clear()
	gs.ID = ""
	gs.IsNew = true

	return nil
}

// load reads the session's values from its row. It reports false for a
// missing or expired row and for data the codecs cannot decode.
func (s *Store) load(ctx context.Context, session *sessions.Session) (bool, error) {
	var data []byte

	err := s.db.QueryRowContext(ctx,
		"SELECT data FROM http_sessions WHERE key = $1 AND expires_on > now()",
		[]byte(session.ID)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to load the session: %w", err)
	}

	if err := securecookie.DecodeMulti(session.Name(), string(data), &session.Values, s.Codecs...); err != nil {
		return false, nil
	}

	return true, nil
}

// save inserts a new session's row or updates an existing one's.
func (s *Store) save(ctx context.Context, session *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.Codecs...)
	if err != nil {
		return fmt.Errorf("failed to encode the session: %w", err)
	}

	now := time.Now()
	maxAge := session.Options.MaxAge
	if maxAge == 0 {
		// A browser-session cookie has no max age; its row still needs one.
		maxAge = DefaultMaxAge
	}

	expiresOn := now.Add(time.Duration(maxAge) * time.Second)

	if session.IsNew {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO http_sessions (key, data, created_on, modified_on, expires_on)
			 VALUES ($1, $2, $3, $4, $5)`,
			[]byte(session.ID), []byte(encoded), now, now, expiresOn)
	} else {
		_, err = s.db.ExecContext(ctx,
			"UPDATE http_sessions SET data = $1, modified_on = $2, expires_on = $3 WHERE key = $4",
			[]byte(encoded), now, expiresOn, []byte(session.ID))
	}

	if err != nil {
		return fmt.Errorf("failed to save the session: %w", err)
	}

	return nil
}

func (s *Store) destroy(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM http_sessions WHERE key = $1", []byte(id)); err != nil {
		return fmt.Errorf("failed to delete the session: %w", err)
	}

	return nil
}
