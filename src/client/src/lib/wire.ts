// Generated from src/server/internal/monitoring. Do not edit.
// Regenerate with:
//   go test ./internal/monitoring -run TestGeneratedWireFile -update
//
// Every name below is part of the wire contract between the server and the
// client. queue.go declares the queues, up_to_date.go the set-aside prefix,
// and listquery.go the page limits.

// The queue order is the display order, worst first. `queues` holds the
// names the effective_queue column stores.
export const queues = ["daily", "up_to_date", "muted", "archived"] as const;
export type Queue = (typeof queues)[number];

// The spelling the client uses in a URL and in a `folder:` chip. The work
// queue is the one queue whose slug differs from its stored name.
export const folderSlugs = ["work", "up_to_date", "muted", "archived"] as const;
export type FolderSlug = (typeof folderSlugs)[number];

// The side screens: every queue but the work queue.
export const sideScreens = ["up_to_date", "muted", "archived"] as const;
export type SideScreen = (typeof sideScreens)[number];

// The opening of the one journal entry that markUpToDate writes.
export const setAsidePrefix = "Set aside";

// The page size the list route defaults to, and the largest it accepts.
export const defaultPageLimit = 25;
export const maxPageLimit = 100;
