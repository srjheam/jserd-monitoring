package email

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/pocketbase/pocketbase/tools/mailer"
)

type Message struct {
	To, From            string
	ReplyTo             []string
	Subject, Text, HTML string
	SensitiveFields     []SensitiveField
}

type SensitiveField struct {
	Name  string
	Value string
}
type Transport interface {
	Send(context.Context, Message) error
}
type SMTPTransport struct{ Mailer mailer.Mailer }

func (t SMTPTransport) Send(_ context.Context, message Message) error {
	from, recipients, err := messageAddresses(message)
	if err != nil {
		return err
	}
	if t.Mailer == nil {
		return fmt.Errorf("SMTP mailer is not configured")
	}
	outgoing := &mailer.Message{From: from, To: recipients, Subject: message.Subject, Text: message.Text, HTML: message.HTML, Headers: replyToHeaders(message.ReplyTo)}
	if err := t.Mailer.Send(outgoing); err != nil {
		return fmt.Errorf("SMTP send failed")
	}
	return nil
}

// messageAddresses reads the sender and the recipients of one message.
func messageAddresses(message Message) (mail.Address, []mail.Address, error) {
	from, err := mail.ParseAddress(message.From)
	if err != nil {
		return mail.Address{}, nil, fmt.Errorf("invalid From address")
	}
	recipients, err := addressList(message.To)
	return *from, recipients, err
}

// addressList reads the recipients of one message.
func addressList(to string) ([]mail.Address, error) {
	parsed, err := mail.ParseAddressList(to)
	if err != nil {
		return nil, fmt.Errorf("invalid To address")
	}
	recipients := make([]mail.Address, len(parsed))
	for i, address := range parsed {
		recipients[i] = *address
	}
	return recipients, nil
}

// replyToHeaders is the Reply-To header of one message. A message with no
// reply address carries no such header.
func replyToHeaders(replyTo []string) map[string]string {
	if len(replyTo) == 0 {
		return map[string]string{}
	}
	return map[string]string{"Reply-To": strings.Join(replyTo, ", ")}
}
