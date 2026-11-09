package ojs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type participantGridResponse struct {
	Status  *bool  `json:"status"`
	Content string `json:"content"`
}

func (c *Client) copyeditorAssigned(ctx context.Context, submissionID string) (bool, error) {
	gridURL, err := c.copyeditorGridURL(submissionID)
	if err != nil {
		return false, err
	}
	content, err := c.fetchParticipantGrid(ctx, gridURL)
	if err != nil {
		return false, err
	}
	return parseCopyeditorGrid(content)
}

func (c *Client) fetchParticipantGrid(ctx context.Context, gridURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gridURL, nil)
	if err != nil {
		return "", fmt.Errorf("OJS participant grid request failed")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OJS participant grid request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || !okStatus(resp.StatusCode) {
		return "", fmt.Errorf("OJS participant grid request failed")
	}
	var grid participantGridResponse
	if err := json.Unmarshal(body, &grid); err != nil || grid.Status == nil || !*grid.Status {
		return "", fmt.Errorf("OJS participant grid response is invalid")
	}
	return grid.Content, nil
}

func (c *Client) copyeditorGridURL(submissionID string) (string, error) {
	return c.participantGridURL(submissionID, copyeditingWorkflowStageID)
}

func (c *Client) participantGridURL(submissionID string, stageID int) (string, error) {
	submissionNumber, err := strconv.Atoi(submissionID)
	if err != nil || submissionNumber <= 0 || stageID <= 0 {
		return "", fmt.Errorf("OJS submission id is invalid")
	}
	gridURL, err := url.Parse(c.contextURL("$$$call$$$/grid/users/stage-participant/stage-participant-grid/fetch-grid"))
	if err != nil {
		return "", fmt.Errorf("OJS participant grid URL is invalid")
	}
	query := gridURL.Query()
	query.Set("submissionId", strconv.Itoa(submissionNumber))
	query.Set("stageId", strconv.Itoa(stageID))
	gridURL.RawQuery = query.Encode()
	return gridURL.String(), nil
}
