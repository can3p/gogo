package mailjet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"testing"

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
