package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	ManifestVersion      = "1"
	ArtifactStatusOK     = "ok"
	ArtifactStatusFailed = "failed"
)

type RawArtifact struct {
	Kind        string    `json:"kind"`
	SourceMode  string    `json:"sourceMode"`
	PathOrURL   string    `json:"pathOrUrl"`
	SHA256      string    `json:"sha256"`
	SizeBytes   int64     `json:"sizeBytes"`
	CollectedAt time.Time `json:"collectedAt"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
}

type Manifest struct {
	Version     string        `json:"version"`
	SourceMode  string        `json:"sourceMode"`
	CollectedAt time.Time     `json:"collectedAt"`
	Artifacts   []RawArtifact `json:"artifacts"`
	Errors      []string      `json:"errors,omitempty"`
}

func (m Manifest) MarshalJSON() ([]byte, error) {
	type manifestAlias Manifest
	if m.Version == "" {
		m.Version = ManifestVersion
	}
	return json.Marshal(manifestAlias(m))
}

func (m Manifest) Artifact(kind string) (RawArtifact, bool) {
	for _, artifact := range m.Artifacts {
		if artifact.Kind == kind {
			return artifact, true
		}
	}
	return RawArtifact{}, false
}

func NewArtifact(kind, sourceMode, pathOrURL string, body []byte, collectedAt time.Time, status string, err error) RawArtifact {
	sum := sha256.Sum256(body)
	artifact := RawArtifact{
		Kind:        kind,
		SourceMode:  sourceMode,
		PathOrURL:   RedactURL(pathOrURL),
		SHA256:      hex.EncodeToString(sum[:]),
		SizeBytes:   int64(len(body)),
		CollectedAt: collectedAt,
		Status:      status,
	}
	if err != nil {
		artifact.Error = RedactError(err.Error())
	}
	return artifact
}

func NewRedactedError(message string) error {
	return fmt.Errorf("%s", RedactError(message))
}

func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || !isAbsoluteURL(parsed) {
		return RedactError(raw)
	}
	parsed.User = nil
	parsed.RawQuery = redactedQuery(parsed.Query())
	return parsed.String()
}

// isAbsoluteURL reports whether a parsed URL names both a scheme and a host.
// Anything else is not a URL this system produced, so it is redacted as plain
// text instead.
func isAbsoluteURL(parsed *url.URL) bool {
	return parsed.Scheme != "" && parsed.Host != ""
}

// redactedQuery replaces the value of every sensitive query param.
func redactedQuery(query url.Values) string {
	for key := range query {
		if containsSensitiveMarker(key) {
			query.Set(key, "redacted")
		}
	}
	return query.Encode()
}

func RedactError(message string) string {
	if index := strings.Index(strings.ToLower(message), "; failed query:"); index >= 0 {
		message = message[:index]
	}
	if containsSensitiveMarker(message) {
		return "redacted"
	}
	return message
}

// sensitiveMarkers are the words that mark a value as a secret.
var sensitiveMarkers = []string{"password", "secret", "cookie", "token"}

// containsSensitiveMarker reports whether a value names a secret.
func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	return slices.ContainsFunc(sensitiveMarkers, func(marker string) bool {
		return strings.Contains(lower, marker)
	})
}
