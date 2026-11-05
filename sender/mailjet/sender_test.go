package mailjet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"testing"
	"time"

	"github.com/can3p/gogo/sender"
	"github.com/can3p/gogo/sender/mailjet"
	"github.com/can3p/gogo/sender/mailjet/config"
	"github.com/stretchr/testify/require"
)

// TestSend_BaseURL sends through a Mailjet mock, the way development and tests
// send through tommy.
func TestSend_BaseURL(t *testing.T) {
	var (
		path, user, password string
		body                 struct {
			Messages []struct {
				To      []struct{ Email string }
				Subject string
			}
		}
	)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		user, password, _ = r.BasicAuth()
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"Messages":[{"Status":"success"}]}`))
	}))
	t.Cleanup(mock.Close)

	snd := mailjet.NewSenderFromConfig(&config.Config{
		ApiKeyPublic:  "public",
		ApiKeyPrivate: "private",
		BaseURL:       mock.URL + "/",
	})

	err := snd.Send(context.Background(), &sender.Mail{
		From:    mail.Address{Address: "from@example.com"},
		To:      []mail.Address{{Address: "to@example.com"}},
		Subject: "Hello",
		Text:    "Hi",
	})

	require.NoError(t, err)
	require.Equal(t, "/v3.1/send", path)
	require.Equal(t, "public", user)
	require.Equal(t, "private", password)
	require.Len(t, body.Messages, 1)
	require.Equal(t, "Hello", body.Messages[0].Subject)
	require.Equal(t, "to@example.com", body.Messages[0].To[0].Email)
}

func TestSend_ErrorStatus(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(mock.Close)

	snd := mailjet.NewSenderFromConfig(&config.Config{ApiKeyPublic: "p", ApiKeyPrivate: "s", BaseURL: mock.URL})

	err := snd.Send(context.Background(), &sender.Mail{To: []mail.Address{{Address: "to@example.com"}}})

	require.Error(t, err)
}

// hangingServer accepts requests and never answers until the test ends.
func hangingServer(t *testing.T) *httptest.Server {
	release := make(chan struct{})
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		mock.Close()
	})
	return mock
}

func TestSend_ContextDeadline(t *testing.T) {
	mock := hangingServer(t)
	snd := mailjet.NewSenderFromConfig(&config.Config{ApiKeyPublic: "p", ApiKeyPrivate: "s", BaseURL: mock.URL, Timeout: time.Minute})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := snd.Send(ctx, &sender.Mail{To: []mail.Address{{Address: "to@example.com"}}})

	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestSend_ClientTimeout(t *testing.T) {
	mock := hangingServer(t)
	snd := mailjet.NewSenderFromConfig(&config.Config{ApiKeyPublic: "p", ApiKeyPrivate: "s", BaseURL: mock.URL, Timeout: 100 * time.Millisecond})

	start := time.Now()
	err := snd.Send(context.Background(), &sender.Mail{To: []mail.Address{{Address: "to@example.com"}}})

	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second)
}

// captureServer is a Mailjet mock that keeps the raw messages of the last request.
func captureServer(t *testing.T) (*httptest.Server, *[]map[string]json.RawMessage) {
	var messages []map[string]json.RawMessage
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Messages []map[string]json.RawMessage }
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages = body.Messages
		_, _ = w.Write([]byte(`{"Messages":[{"Status":"success"}]}`))
	}))
	t.Cleanup(mock.Close)
	return mock, &messages
}

func TestSend_Headers(t *testing.T) {
	mock, messages := captureServer(t)
	snd := mailjet.NewSenderFromConfig(&config.Config{ApiKeyPublic: "p", ApiKeyPrivate: "s", BaseURL: mock.URL})

	headers := map[string]string{
		"List-Unsubscribe":      "<https://example.com/subscriptions/tok/unsubscribe/1>, <https://example.com/subscriptions/tok>",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}

	err := snd.Send(context.Background(), &sender.Mail{
		From:    mail.Address{Address: "from@example.com"},
		To:      []mail.Address{{Address: "to@example.com"}},
		Subject: "Hello",
		Headers: headers,
	})
	require.NoError(t, err)
	require.Len(t, *messages, 1)

	raw, ok := (*messages)[0]["Headers"]
	require.True(t, ok, "Headers key missing")

	var got map[string]string
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, headers, got)
}

func TestSend_NoHeaders(t *testing.T) {
	mock, messages := captureServer(t)
	snd := mailjet.NewSenderFromConfig(&config.Config{ApiKeyPublic: "p", ApiKeyPrivate: "s", BaseURL: mock.URL})

	for _, headers := range []map[string]string{nil, {}} {
		err := snd.Send(context.Background(), &sender.Mail{
			From:    mail.Address{Address: "from@example.com"},
			To:      []mail.Address{{Address: "to@example.com"}},
			Subject: "Hello",
			Headers: headers,
		})
		require.NoError(t, err)
		require.Len(t, *messages, 1)
		require.NotContains(t, (*messages)[0], "Headers")
	}
}

func TestMail_JSONHeaders(t *testing.T) {
	in := sender.Mail{
		To:      []mail.Address{{Address: "to@example.com"}},
		Subject: "Hello",
		Headers: map[string]string{"List-Unsubscribe-Post": "List-Unsubscribe=One-Click"},
	}

	data, err := json.Marshal(in)
	require.NoError(t, err)

	var out sender.Mail
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, in, out)

	// A mail queued before Headers existed, or one without headers.
	data, err = json.Marshal(sender.Mail{Subject: "Hello"})
	require.NoError(t, err)
	require.NotContains(t, string(data), "Headers")

	var old sender.Mail
	require.NoError(t, json.Unmarshal([]byte(`{"From":{"Name":"","Address":"from@example.com"},"To":null,"Cc":null,"Bcc":null,"Subject":"Hello","Text":"Hi","Html":""}`), &old))
	require.Nil(t, old.Headers)
	require.Equal(t, "Hello", old.Subject)
}
