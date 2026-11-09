package monitoring

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateWireFlag = flag.Bool("update", false, "rewrite the generated client wire file instead of checking it")

// generatedWirePath is the client module this test owns. The client must not
// keep its own copy of the queue vocabulary, the set-aside prefix, or the page
// limits: it used to, in four separate modules, and nothing kept them equal.
const generatedWirePath = "../../../client/src/lib/wire.ts"

// TestGeneratedWireFileIsCurrent writes the shared wire constants out as
// TypeScript and fails when the result differs from the file that is checked
// in. It is a test rather than a go:generate step so that it runs on every
// commit, with no tool to remember and no route to call.
//
// When it fails, run it with -update to rewrite the file:
//
//	go test ./internal/monitoring -run TestGeneratedWireFile -update
func TestGeneratedWireFileIsCurrent(t *testing.T) {
	want := renderWireTS()
	if *updateWireFlag {
		if err := os.WriteFile(generatedWirePath, []byte(want), 0o644); err != nil {
			t.Fatalf("write %s: %v", generatedWirePath, err)
		}
		t.Logf("wrote %s", generatedWirePath)
		return
	}
	got, err := os.ReadFile(generatedWirePath)
	if err != nil {
		t.Fatalf("read %s: %v (run this test with -update to create it)", generatedWirePath, err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of date with the server.\nRun: go test ./internal/monitoring -run TestGeneratedWireFile -update", filepath.Clean(generatedWirePath))
	}
}

func renderWireTS() string {
	var out strings.Builder
	out.WriteString("// Generated from src/server/internal/monitoring. Do not edit.\n")
	out.WriteString("// Regenerate with:\n")
	out.WriteString("//   go test ./internal/monitoring -run TestGeneratedWireFile -update\n")
	out.WriteString("//\n")
	out.WriteString("// Every name below is part of the wire contract between the server and the\n")
	out.WriteString("// client. queue.go declares the queues, up_to_date.go the set-aside prefix,\n")
	out.WriteString("// and listquery.go the page limits.\n\n")

	out.WriteString("// The queue order is the display order, worst first. `queues` holds the\n")
	out.WriteString("// names the effective_queue column stores.\n")
	out.WriteString(tsStringUnion("queues", "Queue", queueNames()))

	out.WriteString("\n// The spelling the client uses in a URL and in a `folder:` chip. The work\n")
	out.WriteString("// queue is the one queue whose slug differs from its stored name.\n")
	out.WriteString(tsStringUnion("folderSlugs", "FolderSlug", queueSlugs()))

	out.WriteString("\n// The side screens: every queue but the work queue.\n")
	out.WriteString(tsStringUnion("sideScreens", "SideScreen", sideScreenNames()))

	out.WriteString("\n// The opening of the one journal entry that markUpToDate writes.\n")
	fmt.Fprintf(&out, "export const setAsidePrefix = %q;\n", setAsidePrefix)

	out.WriteString("\n// The page size the list route defaults to, and the largest it accepts.\n")
	fmt.Fprintf(&out, "export const defaultPageLimit = %d;\n", defaultPageLimit)
	fmt.Fprintf(&out, "export const maxPageLimit = %d;\n", maxPageLimit)
	return out.String()
}

// tsStringUnion renders one string list plus the union type read off it, in
// the shape biome formats it to.
func tsStringUnion(constName, typeName string, values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return fmt.Sprintf("export const %s = [%s] as const;\nexport type %s = (typeof %s)[number];\n",
		constName, strings.Join(quoted, ", "), typeName, constName)
}

// queueSlugs returns the client-facing slug of every queue, in display order.
func queueSlugs() []string {
	slugs := make([]string, 0, len(queues))
	for _, entry := range queues {
		slugs = append(slugs, entry.slug)
	}
	return slugs
}

// sideScreenNames returns the queues that have a side screen: every queue the
// work list itself does not show.
func sideScreenNames() []string {
	names := make([]string, 0, len(queues)-1)
	for _, entry := range queues {
		if entry.name != defaultQueue {
			names = append(names, entry.name)
		}
	}
	return names
}
