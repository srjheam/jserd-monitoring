package ojs

import "fmt"

// pageCursor walks an OJS REST collection one page at a time. It holds the
// offset of the next page, and the total every page so far reported.
type pageCursor struct {
	resource    string
	offset      int
	expectedMax int
}

func newPageCursor(resource string) pageCursor {
	return pageCursor{resource: resource, expectedMax: -1}
}

// consistentMax reports whether a page's reported total agrees with the total
// the pages before it reported.
func (p pageCursor) consistentMax(itemsMax int) bool {
	return itemsMax >= 0 && (p.expectedMax < 0 || itemsMax == p.expectedMax)
}

// advance moves the cursor past the page just read. It reports done once every
// item was read. It fails when OJS reports an inconsistent total, or when a
// page carries no item while items remain.
func (p *pageCursor) advance(itemsMax, itemCount int) (bool, error) {
	if !p.consistentMax(itemsMax) {
		return false, fmt.Errorf("REST %s pagination metadata is invalid", p.resource)
	}
	p.expectedMax = itemsMax
	p.offset += itemCount
	if p.offset >= itemsMax {
		return true, nil
	}
	if itemCount == 0 {
		return false, fmt.Errorf("REST %s pagination made no progress", p.resource)
	}
	return false, nil
}
