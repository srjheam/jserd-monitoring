package email

import (
	"fmt"
	"net/mail"
	"strings"
)

func ReviewerReplyTo(editors []EditorContact, fallback string) ([]string, error) {
	result, err := editorReplyToAddresses(editors)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		return result, nil
	}
	return EditorReplyTo(fallback)
}

// editorReplyToAddresses are the Reply-To addresses of the assigned editors. An
// editor whose e-mail does not parse is left out. A line break in a name or an
// address injects a header, so it fails the whole list.
func editorReplyToAddresses(editors []EditorContact) ([]string, error) {
	result := []string{}
	for _, editor := range editors {
		if strings.ContainsAny(editor.Name+editor.Email, "\r\n") {
			return nil, fmt.Errorf("reply-to contains CR/LF")
		}
		address, err := mail.ParseAddress(editor.Email)
		if err != nil {
			continue
		}
		address.Name = editor.Name
		result = append(result, address.String())
	}
	return result, nil
}

func EditorReplyTo(institutional string) ([]string, error) {
	if institutional == "" {
		return nil, nil
	}
	if strings.ContainsAny(institutional, "\r\n") {
		return nil, fmt.Errorf("reply-to contains CR/LF")
	}
	address, err := mail.ParseAddress(institutional)
	if err != nil {
		return nil, fmt.Errorf("invalid institutional reply-to")
	}
	return []string{address.String()}, nil
}
