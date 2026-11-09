package state

import "server/internal/audit"

func redactManifest(manifest audit.Manifest) audit.Manifest {
	redacted := manifest
	redacted.Errors = append([]string(nil), manifest.Errors...)
	for i := range redacted.Errors {
		redacted.Errors[i] = audit.RedactError(redacted.Errors[i])
	}
	redacted.Artifacts = append([]audit.RawArtifact(nil), manifest.Artifacts...)
	for i := range redacted.Artifacts {
		redacted.Artifacts[i].PathOrURL = audit.RedactURL(redacted.Artifacts[i].PathOrURL)
		redacted.Artifacts[i].Error = audit.RedactError(redacted.Artifacts[i].Error)
	}
	return redacted
}
