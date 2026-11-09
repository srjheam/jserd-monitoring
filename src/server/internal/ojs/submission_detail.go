package ojs

import "context"

const (
	copyeditingWorkflowStageID = 4
	copyeditingQueuedStatus    = 1
)

// SubmissionDetail is the single-submission detail endpoint's response,
// narrowed to fields the editorial pipeline's live-enrichment stages need:
// review round and reviewAssignment status, and the assigned copyeditor for
// an active copyediting submission. The bulk GET /submissions call omits
// these fields, so rules needing them share this detail fetch.
type SubmissionDetail struct {
	StageID            int                      `json:"stageId"`
	Status             int                      `json:"status"`
	ReviewRounds       []reviewRoundPayload     `json:"reviewRounds"`
	ReviewAssignments  []reviewAssignmentWindow `json:"reviewAssignments"`
	CopyeditorAssigned *bool                    `json:"-"`
}

// SubmissionDetail fetches one submission's full detail record.
func (c *Client) SubmissionDetail(ctx context.Context, submissionID string) (SubmissionDetail, error) {
	var detail SubmissionDetail
	if _, _, err := c.GetJSON(ctx, "submissions/"+submissionID, &detail); err != nil {
		return SubmissionDetail{}, err
	}
	if detail.StageID == copyeditingWorkflowStageID && detail.Status == copyeditingQueuedStatus {
		assigned, err := c.copyeditorAssigned(ctx, submissionID)
		if err != nil {
			return SubmissionDetail{}, err
		}
		detail.CopyeditorAssigned = &assigned
	}
	return detail, nil
}
