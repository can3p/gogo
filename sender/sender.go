package sender

import (
	"context"
	"net/mail"

	"github.com/volatiletech/sqlboiler/v4/boil"
)

type Sender interface {
	Send(ctx context.Context, exec boil.ContextExecutor, uniqueID string, emailType string, mail *Mail) error
}

type Mail struct {
	From    mail.Address
	To      []mail.Address
	Cc      []mail.Address
	Bcc     []mail.Address
	Subject string
	Text    string
	Html    string
}
