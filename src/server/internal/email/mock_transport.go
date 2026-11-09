package email

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/mail"
	"sort"
	"strings"
)

type MockTransport struct {
	Output io.Writer
	Sent   []Message
}

func (m *MockTransport) Send(_ context.Context, message Message) error {
	safe := pseudonymMessage(message)
	m.Sent = append(m.Sent, safe)
	if m.Output == nil {
		return nil
	}
	_, err := fmt.Fprintf(m.Output, "To: %s\nFrom: %s\nSubject: %s\nText:\n%s\nHTML:\n%s\n", safe.To, safe.From, safe.Subject, safe.Text, safe.HTML)
	return err
}

// pseudonymMessage replaces every address and every sensitive value in a
// message with a stable pseudonym, so nothing real is ever written down.
func pseudonymMessage(message Message) Message {
	safe := message
	safe.To = pseudonymAddress(message.To)
	safe.From = pseudonymAddress(message.From)
	safe.ReplyTo = pseudonymAddresses(message.ReplyTo)
	for _, value := range append([]string{message.To, message.From}, message.ReplyTo...) {
		replaceInBodies(&safe, addressForms(value), pseudonymAddress(value))
	}
	for _, field := range longestValueFirst(message.SensitiveFields) {
		replaceInBodies(&safe, []string{field.Value}, pseudonymField(field))
	}
	safe.SensitiveFields = nil
	return safe
}

// pseudonymAddresses replaces every address of a list.
func pseudonymAddresses(addresses []string) []string {
	safe := make([]string, len(addresses))
	for i, address := range addresses {
		safe[i] = pseudonymAddress(address)
	}
	return safe
}

// addressForms are the written forms one address can take in a body: the whole
// value, and the bare address inside it when the value carries a display name.
func addressForms(value string) []string {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address == value {
		return []string{value}
	}
	return []string{value, address.Address}
}

// replaceInBodies replaces every given form in the subject and in both bodies.
// An empty form is skipped, because replacing it would cut the body apart.
func replaceInBodies(message *Message, forms []string, replacement string) {
	for _, form := range forms {
		if form == "" {
			continue
		}
		message.Subject = strings.ReplaceAll(message.Subject, form, replacement)
		message.Text = strings.ReplaceAll(message.Text, form, replacement)
		message.HTML = strings.ReplaceAll(message.HTML, form, replacement)
	}
}

// longestValueFirst orders the sensitive fields by value length, longest
// first, so a long value is replaced before a shorter value inside it.
func longestValueFirst(fields []SensitiveField) []SensitiveField {
	ordered := append([]SensitiveField(nil), fields...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if len(ordered[i].Value) == len(ordered[j].Value) {
			return ordered[i].Name < ordered[j].Name
		}
		return len(ordered[i].Value) > len(ordered[j].Value)
	})
	return ordered
}

// pseudonymPrefixes maps each sensitive field to the prefix its pseudonym
// carries, so a reader can still tell what a replaced value stood for.
var pseudonymPrefixes = map[string]string{
	"recipient_name": "Pessoa-",
	"submission_id":  "SUB-",
	"title":          "Artigo-",
	"ojs_url":        "https://ojs.example.test/submission/",
	"deadline":       "Prazo-",
	"reviewer_name":  "Revisor-",
}

func pseudonymField(field SensitiveField) string {
	sum := sha256.Sum256([]byte(field.Name + "\x00" + strings.ToLower(strings.TrimSpace(field.Value))))
	token := fmt.Sprintf("%x", sum[:6])
	prefix, known := pseudonymPrefixes[field.Name]
	if !known {
		prefix = "Campo-"
	}
	return prefix + token
}

func pseudonymAddress(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	return fmt.Sprintf("destinatario-%x@example.test", sum[:6])
}
