package editorial

import (
	"server/internal/collections"
	"server/internal/pbfilter"
	"sort"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// This file computes the set of articles a summary e-mail still needs to
// name: the latest findings, minus whatever a chief editor has muted or set
// aside as up to date, in work-list order.

// pendingArticles is the set of articles that still need a chief editor, in
// work list order.
func (s *DeliveryService) pendingArticles() ([]ArticleFinding, error) {
	articles, err := LatestArticleFindings(s.App)
	if err != nil || len(articles) == 0 {
		return nil, err
	}
	return s.dropSetAside(articles)
}

// dropSetAside drops the articles whose submission the chief editor has
// muted or set aside as up to date, then orders the survivors by
// effective_priority and, within a priority, by elapsed days descending
// (oldest submission first), matching the work list order.
func (s *DeliveryService) dropSetAside(articles []ArticleFinding) ([]ArticleFinding, error) {
	tracking, err := s.trackingBySubmission(submissionIDsOf(articles))
	if err != nil {
		return nil, err
	}
	survivors := make([]ArticleFinding, 0, len(articles))
	for _, article := range articles {
		if isSetAside(tracking[article.SubmissionID], article.Findings) {
			continue
		}
		survivors = append(survivors, article)
	}
	sortByWorkListOrder(survivors)
	return survivors, nil
}

// submissionIDsOf lists the submission of each article.
func submissionIDsOf(articles []ArticleFinding) []string {
	ids := make([]string, len(articles))
	for i, article := range articles {
		ids[i] = article.SubmissionID
	}
	return ids
}

// sortByWorkListOrder orders articles by effective priority and, within a
// priority, by elapsed days descending (oldest submission first).
func sortByWorkListOrder(articles []ArticleFinding) {
	sort.SliceStable(articles, func(i, j int) bool {
		pi, pj := PriorityFor(articles[i].Findings), PriorityFor(articles[j].Findings)
		if pi != pj {
			return pi < pj
		}
		return articles[i].DateSubmitted < articles[j].DateSubmitted
	})
}

func (s *DeliveryService) trackingBySubmission(ids []string) (map[string]*core.Record, error) {
	if len(ids) == 0 {
		return map[string]*core.Record{}, nil
	}
	filter, params := pbfilter.IDFilter("ojs_submission_id", ids)
	records, err := s.App.FindRecordsByFilter(collections.SubmissionTracking, filter, "", 0, 0, params)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*core.Record, len(records))
	for _, record := range records {
		result[record.GetString("ojs_submission_id")] = record
	}
	return result, nil
}

// isSetAside reports whether a submission is currently Muted or Up to date.
// Muted holds regardless of findings: it is delegation, not a claim the work
// is fine, and it ends only when a chief editor unmutes it. Up to date holds
// only while the tracking record's stored digest still matches the
// submission's current finding digest and any return date has not passed.
func isSetAside(tracking *core.Record, findings []Finding) bool {
	if tracking == nil {
		return false
	}
	if tracking.GetBool("muted") {
		return true
	}
	return upToDateHolds(tracking, findings)
}

// upToDateHolds reports whether the up-to-date mark still stands: the stored
// digest still matches the submission's current finding digest, and the return
// date has not passed.
func upToDateHolds(tracking *core.Record, findings []Finding) bool {
	digest := tracking.GetString("up_to_date_digest")
	if digest == "" || digest != DigestFor(findings) {
		return false
	}
	until := tracking.GetDateTime("up_to_date_until")
	return until.IsZero() || until.Time().After(time.Now().UTC())
}
