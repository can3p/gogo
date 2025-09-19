package mailjet

import (
	"context"
	"net/mail"
	"os"

	"github.com/can3p/gogo/sender"
	"github.com/can3p/gogo/sender/mailjet/config"
	mailjet "github.com/mailjet/mailjet-apiv3-go/v3"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

type mailjerSender struct {
	client *mailjet.Client
}

var RequiredEnv = []string{"MJ_APIKEY_PUBLIC", "MJ_APIKEY_PRIVATE"}

func NewSender() *mailjerSender {
	return NewSenderFromConfig(&config.Config{
		ApiKeyPublic:  os.Getenv("MJ_APIKEY_PUBLIC"),
		ApiKeyPrivate: os.Getenv("MJ_APIKEY_PRIVATE"),
	})
}

func NewSenderFromConfig(config *config.Config) *mailjerSender {
	mailjetClient := mailjet.NewMailjetClient(config.ApiKeyPublic, config.ApiKeyPrivate)

	return &mailjerSender{
		client: mailjetClient,
	}
}

func toMailjet(addrs []mail.Address) *mailjet.RecipientsV31 {
	if len(addrs) == 0 {
		return nil
	}

	out := mailjet.RecipientsV31{}

	for _, addr := range addrs {
		out = append(out, mailjet.RecipientV31{
			Email: addr.Address,
			Name:  addr.Name,
		})
	}

	return &out

}

func (m *mailjerSender) Send(ctx context.Context, exec boil.ContextExecutor, uniqueID string, emailType string, mail *sender.Mail) error {
	messagesInfo := []mailjet.InfoMessagesV31{
		{
			From: &mailjet.RecipientV31{
				Email: mail.From.Address,
				Name:  mail.From.Name,
			},
			To:       toMailjet(mail.To),
			Cc:       toMailjet(mail.Cc),
			Bcc:      toMailjet(mail.Bcc),
			Subject:  mail.Subject,
			TextPart: mail.Text,
			HTMLPart: mail.Html,
		},
	}

	messages := mailjet.MessagesV31{Info: messagesInfo}
	_, err := m.client.SendMailV31(&messages)

	return err
}
