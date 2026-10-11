package pgstore_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	gsessions "github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/can3p/gogo/sessions/pgstore"
	"github.com/can3p/gogo/testcontainers/postgres"
)

// The fixture was written with antonlindstrom/pgstore's encoding: hash key
// fixtureKey as the only key pair, cookie "sess", session ID
// FIXTURESESSIONID, values {"user_id": "user-123", "csrf_token": "token-456"}.
const (
	fixtureKey    = "gogo-pgstore-fixture-hash-key-32"
	fixtureID     = "FIXTURESESSIONID"
	fixtureCookie = "MTc5MTcyMzA2N3xFd3dBRUVaSldGUlZVa1ZUUlZOVFNVOU9TVVE9fCZa2UOvbDH6Ziiw8JihqhSFblmr8raTxOrlsp1T9DDO"
	fixtureData   = "MTc5MTcyMzA2N3xEWDhFQVFMX2dBQUJFQUVRQUFCU180QUFBZ1p6ZEhKcGJtY01EQUFLWTNOeVpsOTBiMnRsYmdaemRISnBibWNNQ3dBSmRHOXJaVzR0TkRVMkJuTjBjbWx1Wnd3SkFBZDFjMlZ5WDJsa0JuTjBjbWx1Wnd3S0FBaDFjMlZ5TFRFeU13PT18kMDdwYvJQ7rI0ZO8t2rLm15LPgoghh03cJEwgzfTSMc="
)

// migrations is the directory the package ships, so the tests run the
// migration applications install.
const migrations = "migrations"

func TestMain(m *testing.M) {
	code := m.Run()
	_ = postgres.Cleanup()
	os.Exit(code)
}

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	return postgres.New(t, postgres.WithMigrationsDir(migrations)).SQL
}

func newStore(t *testing.T) (*pgstore.Store, *sql.DB) {
	t.Helper()
	db := newDB(t)

	return pgstore.New(db, []byte(fixtureKey)), db
}

func requestWith(cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	return req
}

func countRows(t *testing.T, db *sql.DB) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM http_sessions").Scan(&n))

	return n
}

func insertRow(t *testing.T, db *sql.DB, key, data string, expiresOn time.Time) {
	t.Helper()

	_, err := db.Exec(
		"INSERT INTO http_sessions (key, data, created_on, modified_on, expires_on) VALUES ($1, $2, now(), now(), $3)",
		[]byte(key), []byte(data), expiresOn)
	require.NoError(t, err)
}

func TestReadsPgstoreSessions(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	store.MaxAge(0) // the fixture's securecookie timestamp is fixed
	insertRow(t, db, fixtureID, fixtureData, time.Now().Add(time.Hour))

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.NoError(t, err)
	require.False(t, sess.IsNew)
	require.Equal(t, fixtureID, sess.ID)
	require.Equal(t, map[any]any{"user_id": "user-123", "csrf_token": "token-456"}, sess.Values)
}

func TestOptionsSetSessionOptionsAndCodecMaxAge(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	insertRow(t, db, fixtureID, fixtureData, time.Now().Add(time.Hour))
	store.Options(ginsessions.Options{Path: "/custom", MaxAge: 1, HttpOnly: true})

	// The fixture cookie is older than a second, so the codecs reject it.
	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Empty(t, sess.Values)
	require.Equal(t, &gsessions.Options{Path: "/custom", MaxAge: 1, HttpOnly: true}, sess.Options)
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)

	sess, err := store.New(requestWith(), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Equal(t, &gsessions.Options{Path: "/", MaxAge: pgstore.DefaultMaxAge}, sess.Options)
	require.Equal(t, 30*24*60*60, pgstore.DefaultMaxAge)
}

func TestRoundTripThroughGin(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginsessions.Sessions("sess", store))
	r.GET("/set", func(c *gin.Context) {
		s := ginsessions.Default(c)
		s.Set("user_id", c.Query("user"))
		require.NoError(t, s.Save())
	})
	r.GET("/get", func(c *gin.Context) {
		s := ginsessions.Default(c)
		v, _ := s.Get("user_id").(string)
		c.String(http.StatusOK, "%s|%s", s.ID(), v)
	})

	serve := func(path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		return w
	}

	cookies := serve("/set?user=u1", nil).Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "sess", cookies[0].Name)

	idAndValue := strings.Split(serve("/get", cookies).Body.String(), "|")
	require.Len(t, idAndValue, 2)
	id := idAndValue[0]
	require.Len(t, id, 52, "an ID is 32 random bytes in unpadded base32")
	require.Equal(t, "u1", idAndValue[1])

	var expiresOn time.Time
	require.NoError(t, db.QueryRow("SELECT expires_on FROM http_sessions WHERE key = $1", []byte(id)).Scan(&expiresOn))
	require.WithinDuration(t, time.Now().Add(pgstore.DefaultMaxAge*time.Second), expiresOn, time.Minute)

	// Saving a loaded session updates its row in place.
	cookies2 := serve("/set?user=u2", cookies).Result().Cookies()
	require.Equal(t, 1, countRows(t, db))
	require.Equal(t, id+"|u2", serve("/get", cookies2).Body.String())

	// Without a cookie there is no session.
	require.Equal(t, "|", serve("/get", nil).Body.String())
}

func TestBrowserSessionCookieIsReadBack(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)
	store.MaxAge(0)

	sess, err := store.New(requestWith(), "sess")
	require.NoError(t, err)
	sess.Values["k"] = "v"
	w := httptest.NewRecorder()
	require.NoError(t, store.Save(requestWith(), w, sess))

	loaded, err := store.New(requestWith(w.Result().Cookies()...), "sess")
	require.NoError(t, err)
	require.False(t, loaded.IsNew, "a MaxAge 0 session must not expire at once")
	require.Equal(t, "v", loaded.Values["k"])
}

func TestNegativeMaxAgeDeletesSession(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)

	sess, err := store.New(requestWith(), "sess")
	require.NoError(t, err)
	sess.Values["k"] = "v"
	w := httptest.NewRecorder()
	require.NoError(t, store.Save(requestWith(), w, sess))
	require.Equal(t, 1, countRows(t, db))

	loaded, err := store.New(requestWith(w.Result().Cookies()...), "sess")
	require.NoError(t, err)
	require.False(t, loaded.IsNew)

	loaded.Options.MaxAge = -1
	w2 := httptest.NewRecorder()
	require.NoError(t, store.Save(requestWith(), w2, loaded))
	require.Equal(t, 0, countRows(t, db))

	cookies := w2.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "sess", cookies[0].Name)
	require.Empty(t, cookies[0].Value)
	require.Negative(t, cookies[0].MaxAge, "the cookie must expire now")
}

func TestExpiredRowIsMissing(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	store.MaxAge(0)
	insertRow(t, db, fixtureID, fixtureData, time.Now().Add(-time.Minute))

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Empty(t, sess.ID, "a fresh ID is minted on save")
	require.Empty(t, sess.Values)
}

func TestMissingRowIsNew(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)
	store.MaxAge(0)

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Empty(t, sess.ID)
	require.Empty(t, sess.Values)
}

func TestUndecodableRowIsNew(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	store.MaxAge(0)
	insertRow(t, db, fixtureID, "garbage", time.Now().Add(time.Hour))

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Empty(t, sess.ID)
	require.Empty(t, sess.Values)
}

func TestTamperedCookieIsNew(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	store.MaxAge(0)
	insertRow(t, db, fixtureID, fixtureData, time.Now().Add(time.Hour))

	tampered := fixtureCookie[:len(fixtureCookie)-2] + "AA"
	require.NotEqual(t, fixtureCookie, tampered)

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: tampered}), "sess")
	require.NoError(t, err)
	require.True(t, sess.IsNew)
	require.Empty(t, sess.ID)
	require.Empty(t, sess.Values)
}

func TestMaxLength(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	store.MaxLength(16)

	sess, err := store.New(requestWith(), "sess")
	require.NoError(t, err)
	sess.Values["k"] = strings.Repeat("v", 100)
	require.ErrorContains(t, store.Save(requestWith(), httptest.NewRecorder(), sess), "encode")
	require.Equal(t, 0, countRows(t, db))
}

func TestDatabaseErrors(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	store := pgstore.New(db, []byte(fixtureKey))
	store.MaxAge(0)
	require.NoError(t, db.Close())

	sess, err := store.New(requestWith(&http.Cookie{Name: "sess", Value: fixtureCookie}), "sess")
	require.ErrorContains(t, err, "failed to load the session")
	require.True(t, sess.IsNew, "gin-contrib uses the session even with an error")
	require.Empty(t, sess.ID)

	require.ErrorContains(t, store.Save(requestWith(), httptest.NewRecorder(), sess), "failed to save the session")

	sess.Options.MaxAge = -1
	require.ErrorContains(t, store.Save(requestWith(), httptest.NewRecorder(), sess), "failed to delete the session")

	_, err = store.DeleteExpired(context.Background())
	require.ErrorContains(t, err, "failed to delete expired sessions")
}

func TestRegenerate(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginsessions.Sessions("sess", store))
	r.GET("/visit", func(c *gin.Context) {
		s := ginsessions.Default(c)
		s.Set("csrf_token", "before")
		require.NoError(t, s.Save())
	})
	r.GET("/login", func(c *gin.Context) {
		s := ginsessions.Default(c)
		oldID := s.ID()
		require.NotEmpty(t, oldID)
		require.NoError(t, pgstore.Regenerate(c.Request.Context(), s))
		require.Nil(t, s.Get("csrf_token"), "the old values must not carry over")
		require.Empty(t, s.ID())
		require.Equal(t, 0, countRows(t, db), "the old row must be deleted at once")
		s.Set("user", "user-id-1")
		require.NoError(t, s.Save())
		require.NotEqual(t, oldID, s.ID())
	})

	serve := func(path string, cookies []*http.Cookie) []*http.Cookie {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, func() *http.Request {
			req := requestWith(cookies...)
			req.URL.Path = path

			return req
		}())
		require.Equal(t, http.StatusOK, w.Code)

		return w.Result().Cookies()
	}

	before := serve("/visit", nil)
	require.Len(t, before, 1)

	after := serve("/login", before)
	require.Len(t, after, 1)
	require.NotEqual(t, before[0].Value, after[0].Value, "login must issue a new session cookie")
	require.Equal(t, 1, countRows(t, db))

	load := func(cookies []*http.Cookie) *gsessions.Session {
		sess, err := store.New(requestWith(cookies...), "sess")
		require.NoError(t, err)

		return sess
	}

	require.True(t, load(before).IsNew, "the old session must be gone")
	require.Equal(t, "user-id-1", load(after).Values["user"])
}

// ginSession is embedded under its own name, so that nilGorilla may have a
// Session method.
type ginSession = ginsessions.Session

type notGorilla struct{ ginSession }

type nilGorilla struct{ ginSession }

func (nilGorilla) Session() *gsessions.Session { return nil }

func TestRegenerateErrors(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, pgstore.Regenerate(t.Context(), notGorilla{}), "not backed by gorilla/sessions")
	require.ErrorContains(t, pgstore.Regenerate(t.Context(), nilGorilla{}), "no session to regenerate")
}

func TestDeleteExpired(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	insertRow(t, db, "expired1", "d", time.Now().Add(-time.Hour))
	insertRow(t, db, "expired2", "d", time.Now().Add(-time.Second))
	insertRow(t, db, "live", "d", time.Now().Add(time.Hour))

	n, err := store.DeleteExpired(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, n)

	var key []byte
	require.NoError(t, db.QueryRow("SELECT key FROM http_sessions").Scan(&key))
	require.Equal(t, "live", string(key))
}

func TestCleanup(t *testing.T) {
	t.Parallel()

	store, db := newStore(t)
	insertRow(t, db, "expired", "d", time.Now().Add(-time.Hour))
	insertRow(t, db, "live", "d", time.Now().Add(time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		store.Cleanup(ctx, 10*time.Millisecond, zap.NewNop())
		close(done)
	}()

	require.Eventually(t, func() bool { return countRows(t, db) == 1 }, 5*time.Second, 10*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Cleanup did not stop when its context was cancelled")
	}
}

func TestCleanupLogsFailures(t *testing.T) {
	t.Parallel()

	db := newDB(t)
	store := pgstore.New(db, []byte(fixtureKey))
	require.NoError(t, db.Close())

	core, logs := observer.New(zap.ErrorLevel)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		store.Cleanup(ctx, 10*time.Millisecond, zap.New(core))
		close(done)
	}()

	require.Eventually(t, func() bool { return logs.Len() > 0 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done

	entry := logs.All()[0]
	require.Equal(t, "pgstore: failed to delete expired sessions", entry.Message)
	require.Contains(t, entry.ContextMap()["error"], "failed to delete expired sessions")
}
