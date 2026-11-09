package ojs

import (
	"server/internal/audit"
	"time"
)

func StoreDownloadedArtifact(store audit.ArtifactStore, runID, kind, ext, sourceURL string, body []byte, collectedAt time.Time) (audit.RawArtifact, error) {
	path, err := store.WriteRaw(runID, kind, ext, body)
	if err != nil {
		return audit.NewArtifact(kind, "live", sourceURL, body, collectedAt, audit.ArtifactStatusFailed, err), err
	}
	return audit.NewArtifact(kind, "live", path, body, collectedAt, audit.ArtifactStatusOK, nil), nil
}
