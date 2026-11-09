package ojs

import (
	"context"
	"fmt"
	"strconv"
)

// maxSubmissionPageSize is the largest page OJS will serve for the
// submissions collection.
const maxSubmissionPageSize = 100

type SubmissionFileCount struct {
	SubmissionID       string
	SubmissionProgress int
	DateLastActivity   string
}

type SubmissionPage struct {
	Body      []byte
	SourceURL string
	Items     []SubmissionFileCount
	ItemsMax  int
}

func (c *Client) ListSubmissions(ctx context.Context, count int) ([]SubmissionPage, error) {
	if err := validatePageCount(count); err != nil {
		return nil, err
	}
	return c.submissionPages(ctx, count)
}

// validatePageCount rejects a page size OJS does not serve.
func validatePageCount(count int) error {
	if count < 1 || count > maxSubmissionPageSize {
		return fmt.Errorf("submission page count must be between 1 and %d", maxSubmissionPageSize)
	}
	return nil
}

// submissionPages walks the whole OJS submissions collection. A page OJS did
// answer stays in the result for the audit trail even when the page itself
// failed; a page rejected before OJS ever replied does not.
func (c *Client) submissionPages(ctx context.Context, count int) ([]SubmissionPage, error) {
	pages := []SubmissionPage{}
	cursor := newPageCursor("submissions")
	for more := true; more; {
		page, answered, err := c.submissionPage(ctx, count, cursor.offset)
		if err != nil {
			if !answered {
				return nil, err
			}
			return append(pages, page), err
		}
		pages = append(pages, page)
		done, err := cursor.advance(page.ItemsMax, len(page.Items))
		if err != nil {
			return nil, err
		}
		more = !done
	}
	return pages, nil
}

// submissionPage reads one page of the OJS submissions collection. It reports
// answered as true once OJS replied, even when the reply itself failed, so the
// caller can keep the raw body for the audit trail.
func (c *Client) submissionPage(ctx context.Context, count, offset int) (page SubmissionPage, answered bool, err error) {
	var response struct {
		Items []struct {
			ID                 int64  `json:"id"`
			SubmissionProgress int    `json:"submissionProgress"`
			DateLastActivity   string `json:"dateLastActivity"`
		} `json:"items"`
		ItemsMax int `json:"itemsMax"`
	}
	body, sourceURL, err := c.GetJSON(ctx, fmt.Sprintf("submissions?count=%d&offset=%d", count, offset), &response)
	if err != nil {
		return SubmissionPage{Body: body, SourceURL: sourceURL}, true, err
	}
	page = SubmissionPage{Body: body, SourceURL: sourceURL, ItemsMax: response.ItemsMax, Items: make([]SubmissionFileCount, 0, len(response.Items))}
	for _, item := range response.Items {
		if item.ID <= 0 {
			return SubmissionPage{}, false, fmt.Errorf("REST submission id is invalid")
		}
		page.Items = append(page.Items, SubmissionFileCount{SubmissionID: strconv.FormatInt(item.ID, 10), SubmissionProgress: item.SubmissionProgress, DateLastActivity: item.DateLastActivity})
	}
	return page, true, nil
}
