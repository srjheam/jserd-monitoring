package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewArtifactRecordsAuditMetadata(t *testing.T) {
	collectedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	body := []byte("Submission ID,Title\n42,Example\n")

	artifact := NewArtifact("articles", "fixtures", "data/examples/articles-JSERD-20260512.csv", body, collectedAt, ArtifactStatusOK, nil)

	if artifact.Kind != "articles" {
		t.Fatalf("kind = %q, want articles", artifact.Kind)
	}
	if artifact.SourceMode != "fixtures" {
		t.Fatalf("sourceMode = %q, want fixtures", artifact.SourceMode)
	}
	if artifact.PathOrURL != "data/examples/articles-JSERD-20260512.csv" {
		t.Fatalf("pathOrUrl = %q", artifact.PathOrURL)
	}
	sum := sha256.Sum256(body)
	if artifact.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %q, want %q", artifact.SHA256, hex.EncodeToString(sum[:]))
	}
	if artifact.SizeBytes != int64(len(body)) {
		t.Fatalf("sizeBytes = %d, want %d", artifact.SizeBytes, len(body))
	}
	if !artifact.CollectedAt.Equal(collectedAt) {
		t.Fatalf("collectedAt = %s, want %s", artifact.CollectedAt, collectedAt)
	}
	if artifact.Status != ArtifactStatusOK {
		t.Fatalf("status = %q, want ok", artifact.Status)
	}
}

func TestManifestJSONOmitsSecretsFromURLsAndErrors(t *testing.T) {
	collectedAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	artifact := NewArtifact(
		"rest-submissions",
		"live",
		"https://editor:secret@journals-sol.sbc.org.br/index.php/jserd/api/v1/submissions?cookie=abc123&count=1",
		[]byte(`{"items":[]}`),
		collectedAt,
		ArtifactStatusFailed,
		NewRedactedError("download failed for editor with password secret and cookie abc123"),
	)
	manifest := Manifest{
		SourceMode:  "live",
		CollectedAt: collectedAt,
		Artifacts:   []RawArtifact{artifact},
	}

	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	text := string(encoded)
	for _, secret := range []string{"secret", "abc123", "editor:"} {
		if strings.Contains(text, secret) {
			t.Fatalf("manifest leaked %q in %s", secret, text)
		}
	}
	if !strings.Contains(text, "redacted") {
		t.Fatalf("expected redaction marker in manifest JSON, got %s", text)
	}
}

func TestRedactURLRemovesCredentialsAndCookieQuery(t *testing.T) {
	got := RedactURL("https://editor:secret@journals-sol.sbc.org.br/index.php/jserd/management/tools/report?pluginName=ArticleReportPlugin&cookie=abc123")
	if strings.Contains(got, "editor") || strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
		t.Fatalf("redacted URL still contains secret material: %s", got)
	}
	if !strings.Contains(got, "pluginName=ArticleReportPlugin") {
		t.Fatalf("redacted URL lost non-secret query: %s", got)
	}
}

func TestRedactErrorRemovesFailedQuery(t *testing.T) {
	got := RedactError("find previous snapshot: context deadline exceeded; failed query: SELECT snapshots.* FROM snapshots")
	if got != "find previous snapshot: context deadline exceeded" {
		t.Fatalf("RedactError() = %q", got)
	}
}
