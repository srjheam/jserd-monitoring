package collector

import (
	"context"
	"fmt"
	"server/internal/audit"
	"server/internal/config"
	"server/internal/normalizer"
	"server/internal/ojs"
)

const submissionPageSize = 100

// downloadSubmissionPages reads the REST submission list, page by page, and
// stores every page OJS answered with.
func (r *liveRun) downloadSubmissionPages(ctx context.Context) ([]ojs.SubmissionPage, error) {
	pages, listErr := r.client.ListSubmissions(ctx, submissionPageSize)
	for index, page := range pages {
		if err := r.storeSubmissionPage(index, page, pageFailure(listErr, index, len(pages))); err != nil {
			r.result.addError(err)
			return nil, err
		}
	}
	if listErr != nil {
		r.result.addError(listErr)
		return nil, listErr
	}
	return pages, nil
}

// pageFailure names the failure to record on one page of the submission list.
// Only the last page of a failed listing carries one, because OJS answered
// every earlier page in full.
func pageFailure(listErr error, index, total int) error {
	if listErr != nil && index == total-1 {
		return listErr
	}
	return nil
}

// storeSubmissionPage stores one page of the REST submission list. A page that
// carries a failure is recorded as a failed artifact instead.
func (r *liveRun) storeSubmissionPage(index int, page ojs.SubmissionPage, pageErr error) error {
	kind := submissionPageKind(index)
	artifact, storeErr := ojs.StoreDownloadedArtifact(r.store, r.runID, kind, "json", page.SourceURL, page.Body, r.collector.now())
	if pageErr != nil {
		artifact = audit.NewArtifact(kind, string(config.SourceModeLive), page.SourceURL, page.Body, r.collector.now(), audit.ArtifactStatusFailed, pageErr)
		storeErr = nil
	}
	if storeErr != nil {
		return storeErr
	}
	r.result.addArtifact(artifact)
	return nil
}

// submissionPageKind names one page of the REST submission list, by the offset
// it starts at.
func submissionPageKind(index int) string {
	if index == 0 {
		return "rest-submissions"
	}
	return fmt.Sprintf("rest-submissions-%06d", index*submissionPageSize)
}

func applySubmissionMetadata(submissions []normalizer.Submission, pages []ojs.SubmissionPage) error {
	progress, activity, err := collectSubmissionMetadata(pages)
	if err != nil {
		return err
	}
	applyLastActivity(submissions, activity)
	applyProgress(submissions, progress)
	return nil
}

func collectSubmissionMetadata(pages []ojs.SubmissionPage) (map[string]int, map[string]string, error) {
	progress := map[string]int{}
	activity := map[string]string{}
	seen := map[string]bool{}
	for _, item := range flattenSubmissionItems(pages) {
		if item.SubmissionID == "" || seen[item.SubmissionID] {
			return nil, nil, fmt.Errorf("invalid submission evidence")
		}
		seen[item.SubmissionID] = true
		progress[item.SubmissionID] = item.SubmissionProgress
		activity[item.SubmissionID] = item.DateLastActivity
	}
	return progress, activity, nil
}

func flattenSubmissionItems(pages []ojs.SubmissionPage) []ojs.SubmissionFileCount {
	items := make([]ojs.SubmissionFileCount, 0)
	for _, page := range pages {
		items = append(items, page.Items...)
	}
	return items
}

func applyProgress(submissions []normalizer.Submission, progress map[string]int) {
	for i := range submissions {
		if value, exists := progress[submissions[i].OJSSubmissionID]; exists {
			submissions[i].SubmissionProgress = value
		}
	}
}

// applyLastActivity copies OJS's own last-activity timestamp onto each
// submission. It stays a separate function so applySubmissionMetadata's own
// call site reads as one step per concern.
func applyLastActivity(submissions []normalizer.Submission, activity map[string]string) {
	for i := range submissions {
		if value := activity[submissions[i].OJSSubmissionID]; value != "" {
			submissions[i].DateLastActivity = value
		}
	}
}
