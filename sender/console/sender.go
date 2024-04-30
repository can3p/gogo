package console

import (
	"bytes"
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/can3p/gogo/sender"
)

type consoleSender struct{}

func NewSender() *consoleSender {
	return &consoleSender{}
}

func serializeAddr(addr []mail.Address) string {
	out := []string{}

	for _, addr := range addr {
		out = append(out, addr.String())
	}

	return strings.Join(out, ", ")
}

func (m *consoleSender) MailToString(mail *sender.Mail) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\n", mail.From.String()))

	if len(mail.To) > 0 {
		buf.WriteString(fmt.Sprintf("To: %s\n", serializeAddr(mail.To)))
	}
	if len(mail.Cc) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\n", serializeAddr(mail.Cc)))
	}
	if len(mail.Bcc) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\n", serializeAddr(mail.Bcc)))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\n", mail.Subject))

	if len(mail.Text) > 0 {
		buf.WriteString(fmt.Sprintf("Text:\n %s\n", mail.Text))
	} else {
		buf.WriteString("Text: <none>\n")
	}

	if len(mail.Html) > 0 {
		buf.WriteString(fmt.Sprintf("HTML:\n %s\n", mail.Html))
	} else {
		buf.WriteString("HTML: <none>\n")
	}

	return buf.String()
}

func (m *consoleSender) Send(ctx context.Context, mail *sender.Mail) error {
	fmt.Println("DEBUG Dump of email, nothing was sent!")
	fmt.Println(m.MailToString(mail))

	return nil
}
