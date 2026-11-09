package editorial

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/pbfilter"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// FindingNormal is the rank of a submission that carries no finding. It is not
// a finding kind: the evaluator never produces it.
const FindingNormal FindingKind = "normal"

// Rank returns the worst finding kind of the set, or FindingNormal when the set
// holds no known kind.
func Rank(findings []Finding) string {
	best := len(catalog)
	for _, finding := range findings {
		if rank := rankOf(finding.Kind); rank < best {
			best = rank
		}
	}
	if best == len(catalog) {
		return string(FindingNormal)
	}
	return string(catalog[best].Kind)
}

// Kinds returns every finding kind the evaluator can produce, worst first.
func Kinds() []FindingKind {
	kinds := make([]FindingKind, len(catalog))
	for i, entry := range catalog {
		kinds[i] = entry.Kind
	}
	return kinds
}

// PriorityFor ranks a submission by the worst of its current finding kinds,
// 1 being the worst, matching the order of Kinds(). A submission with no
// findings ranks one past the worst known kind.
func PriorityFor(findings []Finding) int {
	worst := Rank(findings)
	if worst == string(FindingNormal) {
		return len(catalog) + 1
	}
	return rankOf(FindingKind(worst)) + 1
}

// DigestFor hashes the sorted list of a submission's current finding kinds.
// It deliberately excludes deadlines and counts, so a finding set with the
// same kinds always produces the same digest.
func DigestFor(findings []Finding) string {
	kinds := make([]string, len(findings))
	for i, finding := range findings {
		kinds[i] = string(finding.Kind)
	}
	sort.Strings(kinds)
	sum := sha256.Sum256([]byte(strings.Join(kinds, ",")))
	return hex.EncodeToString(sum[:])
}

// LatestFindings returns the current findings of every submission that
// carries at least one, keyed by OJS submission id. editorial_findings is
// always fully replaced together every cycle, so what it holds is
// unconditionally the latest, with no run or status to filter by.
func LatestFindings(app core.App) (map[string][]Finding, error) {
	return findingsBySubmission(app, nil)
}

// LatestFindingsFor is LatestFindings scoped to the given submission ids, for
// callers that only need the findings of one page of submissions instead of
// every editorial_findings row.
func LatestFindingsFor(app core.App, ids []string) (map[string][]Finding, error) {
	if len(ids) == 0 {
		return map[string][]Finding{}, nil
	}
	return findingsBySubmission(app, ids)
}

// LatestArticleFindings returns the current findings of every submission
// that carries at least one, in the same ArticleFinding shape the evaluator
// produces (title, URL, date submitted included), for callers that need the
// full article rather than just its findings by submission id.
func LatestArticleFindings(app core.App) ([]ArticleFinding, error) {
	records, err := app.FindRecordsByFilter(collections.EditorialFindings, "", "date_submitted,id", 0, 0)
	if err != nil {
		return nil, err
	}
	result := make([]ArticleFinding, 0, len(records))
	for _, record := range records {
		payload, err := decodeFindingPayload(record)
		if err != nil {
			return nil, err
		}
		result = append(result, ArticleFinding{SubmissionID: record.GetString("submission_key"), Title: payload.Title, URL: payload.URL, DateSubmitted: record.GetString("date_submitted"), Findings: payload.Findings})
	}
	return result, nil
}

func findingsBySubmission(app core.App, ids []string) (map[string][]Finding, error) {
	records, err := findingRecords(app, ids)
	if err != nil {
		return nil, err
	}
	result := map[string][]Finding{}
	for _, record := range records {
		payload, err := decodeFindingPayload(record)
		if err != nil {
			return nil, err
		}
		result[record.GetString("submission_key")] = payload.Findings
	}
	return result, nil
}

// findingRecords reads the stored findings. A nil id list reads every one.
func findingRecords(app core.App, ids []string) ([]*core.Record, error) {
	filter, params := "", dbx.Params{}
	if ids != nil {
		filter, params = pbfilter.IDFilter("submission_key", ids)
	}
	return app.FindRecordsByFilter(collections.EditorialFindings, filter, "", 0, 0, params)
}

func decodeFindingPayload(record *core.Record) (findingPayload, error) {
	var payload findingPayload
	if err := json.Unmarshal([]byte(record.GetString("payload")), &payload); err != nil {
		return findingPayload{}, fmt.Errorf("decode editorial finding %s: %w", record.Id, err)
	}
	return payload, nil
}
