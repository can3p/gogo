package sender

import (
	"context"
	"net/mail"
)

// Sender delivers a mail. Queueing it in the database, to send it only if a
// transaction commits, is the application's business: a queue is a
// repository that hands stored mails to a Sender.
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
