package ojs

import (
	"context"
	"fmt"
)

type UserGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserSummary struct {
	ID     int64       `json:"id"`
	Name   string      `json:"fullName"`
	Email  string      `json:"email"`
	Groups []UserGroup `json:"groups"`
}

// userPage is one page of the OJS users collection.
type userPage struct {
	Items    []UserSummary `json:"items"`
	ItemsMax int           `json:"itemsMax"`
}

func (c *Client) ListUserGroups(ctx context.Context) ([]UserSummary, error) {
	users := []UserSummary{}
	cursor := newPageCursor("users")
	for more := true; more; {
		const count = 100
		var page userPage
		_, _, err := c.GetJSON(ctx, fmt.Sprintf("users?count=%d&offset=%d", count, cursor.offset), &page)
		if err != nil {
			return nil, err
		}
		users = append(users, page.Items...)
		done, err := cursor.advance(page.ItemsMax, len(page.Items))
		if err != nil {
			return nil, err
		}
		more = !done
	}
	return users, nil
}
