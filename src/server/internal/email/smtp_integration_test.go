package email_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"server/internal/collections"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/mailer"
	mail "server/internal/email"
)

type mailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type mailpitMessage struct {
	ID      string           `json:"ID"`
	From    mailpitAddress   `json:"From"`
	To      []mailpitAddress `json:"To"`
	Subject string           `json:"Subject"`
}

func TestSMTPDockerIntegration(t *testing.T) {
	if os.Getenv("SMTP_INTEGRATION") != "1" {
		t.Skip("set SMTP_INTEGRATION=1 via scripts/test-smtp-integration.sh")
	}
	app, apiURL := runSMTPIntegrationWorker(t)

	message := waitForMailpitMessage(t, apiURL)
	if message.From.Address != testFromAddress || len(message.To) != 1 || message.To[0].Address != "editor@journal.test" || !strings.HasPrefix(message.Subject, "[JSERD]") {
		t.Fatalf("unexpected envelope: %#v", message)
	}
	assertMailpitDetailAndRaw(t, apiURL, message.ID)
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 || attempts[0].GetString("status") != statusSent || attempts[0].GetString("result_class") != "success" {
		t.Fatalf("attempts=%#v err=%v", attempts, err)
	}
}

// runSMTPIntegrationWorker sends the one message this test observes and
// returns the seeded app and the Mailpit API base URL.
func runSMTPIntegrationWorker(t *testing.T) (*tests.TestApp, string) {
	t.Helper()
	host := envOr(t, "SMTP_HOST", "127.0.0.1")
	port, err := strconv.Atoi(envOr(t, "SMTP_PORT", "1025"))
	if err != nil {
		t.Fatal(err)
	}
	apiURL := strings.TrimRight(envOr(t, "MAILPIT_API_URL", "http://127.0.0.1:8025"), "/")

	app := seedRequest(t, "editor@journal.test")
	transport := mail.SMTPTransport{Mailer: &mailer.SMTPClient{Host: host, Port: port, TLS: false}}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "smtp-integration", From: "JSERD <noreply@jserd.test>", ReplyTo: []string{"editorial@jserd.test"}}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	return app, apiURL
}

func assertMailpitDetailAndRaw(t *testing.T, apiURL, messageID string) {
	t.Helper()
	detail := getMailpitJSON(t, apiURL+"/api/v1/message/"+messageID)
	serialized, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	body := string(serialized)
	for _, expected := range []string{"editorial@jserd.test", testSubmissionTitle, testSubmissionURL} {
		if !strings.Contains(body, expected) {
			t.Fatalf("mail detail missing %q: %s", expected, body)
		}
	}
	raw := getMailpitText(t, apiURL+"/api/v1/message/"+messageID+"/raw")
	for _, expected := range []string{"Content-Type: multipart/alternative", "Content-Type: text/plain", "Content-Type: text/html", "Reply-To: editorial@jserd.test"} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("raw message missing %q: %s", expected, raw)
		}
	}
}

func getMailpitText(t *testing.T, url string) string {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s", url, response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func waitForMailpitMessage(t *testing.T, apiURL string) mailpitMessage {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var response struct {
			Messages []mailpitMessage `json:"messages"`
		}
		decodeMailpitJSON(t, apiURL+"/api/v1/messages", &response)
		if len(response.Messages) == 1 {
			return response.Messages[0]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Mailpit did not receive exactly one message")
	return mailpitMessage{}
}

func getMailpitJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	result := map[string]any{}
	decodeMailpitJSON(t, url, &result)
	return result
}

func decodeMailpitJSON(t *testing.T, url string, target any) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s", url, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func envOr(t *testing.T, key, fallback string) string {
	t.Helper()
	if value := os.Getenv(key); value != "" {
		return value
	}
	if fallback == "" {
		t.Fatalf("%s is required", key)
	}
	return fallback
}
