package console

import (
	"context"
	"net/mail"

	"github.com/can3p/gogo/sender"
	"go.uber.org/zap"
)

// consoleSender logs mails instead of sending them, for development.
type consoleSender struct {
	logger *zap.Logger
}

func NewSender(logger *zap.Logger) *consoleSender {
	return &consoleSender{logger: logger}
}

func addresses(addrs []mail.Address) []string {
	out := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		out = append(out, addr.String())
	}

	return out
}

func (m *consoleSender) Send(ctx context.Context, mail *sender.Mail) error {
	m.logger.Info("Mail not sent, console sender",
		zap.Stringer("from", &mail.From),
		zap.Strings("to", addresses(mail.To)),
		zap.Strings("cc", addresses(mail.Cc)),
		zap.Strings("bcc", addresses(mail.Bcc)),
		zap.String("subject", mail.Subject),
		zap.String("text", mail.Text),
		zap.String("html", mail.Html),
	)

	return nil
}
