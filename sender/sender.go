package sender

import (
	"context"
	"net/mail"
)

type Sender interface {
	Send(ctx context.Context, mail *Mail) error
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
