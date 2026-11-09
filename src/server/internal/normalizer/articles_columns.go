package normalizer

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	authorHeaderRE         = regexp.MustCompile(`^Given Name \(Author ([0-9]+)\)$`)
	editorHeaderRE         = regexp.MustCompile(`^Given Name \(Editor ([0-9]+)\)$`)
	editorDecisionHeaderRE = regexp.MustCompile(`^Editor Decision ([0-9]+)\s+\(Editor ([0-9]+)\)$`)
)

// editorDecisionHeaderGroups is the match count FindStringSubmatch returns for
// editorDecisionHeaderRE: the full match plus its two capture groups.
const editorDecisionHeaderGroups = 3

// articleColumnLayout is the repeated-column layout of one Articles report:
// which author numbers, editor numbers and decision numbers it carries.
type articleColumnLayout struct {
	authors   []int
	editors   []int
	decisions map[int][]int
}

func discoverSequences(headers map[string]int, re *regexp.Regexp, group int) []int {
	seen := map[int]bool{}
	for header := range headers {
		if seq, ok := headerSequence(re, header, group); ok {
			seen[seq] = true
		}
	}
	return sortedKeys(seen)
}

// headerSequence reads the repeated-column number a header carries. A header
// the pattern does not match carries none.
func headerSequence(re *regexp.Regexp, header string, group int) (int, bool) {
	matches := re.FindStringSubmatch(header)
	if len(matches) <= group {
		return 0, false
	}
	seq, err := strconv.Atoi(matches[group])
	return seq, err == nil
}

// sortedKeys lists the members of a set of numbers in increasing order.
func sortedKeys(set map[int]bool) []int {
	keys := make([]int, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func discoverEditorDecisions(headers map[string]int) map[int][]int {
	seen := map[int]map[int]bool{}
	for header := range headers {
		editorSeq, decisionSeq, ok := editorDecisionSequences(header)
		if !ok {
			continue
		}
		if seen[editorSeq] == nil {
			seen[editorSeq] = map[int]bool{}
		}
		seen[editorSeq][decisionSeq] = true
	}
	result := map[int][]int{}
	for editorSeq, decisionSet := range seen {
		result[editorSeq] = sortedKeys(decisionSet)
	}
	return result
}

// editorDecisionSequences reads the editor number and the decision number an
// "Editor Decision N  (Editor M)" header carries.
func editorDecisionSequences(header string) (editorSeq, decisionSeq int, ok bool) {
	matches := editorDecisionHeaderRE.FindStringSubmatch(header)
	if len(matches) != editorDecisionHeaderGroups {
		return 0, 0, false
	}
	decisionSeq, decisionErr := strconv.Atoi(matches[1])
	editorSeq, editorErr := strconv.Atoi(matches[2])
	return editorSeq, decisionSeq, decisionErr == nil && editorErr == nil
}

func authorColumn(name string, seq int) string {
	return strings.TrimSpace(name) + " (Author " + strconv.Itoa(seq) + ")"
}

func editorColumn(name string, seq int) string {
	return strings.TrimSpace(name) + " (Editor " + strconv.Itoa(seq) + ")"
}

func editorDecisionColumn(decisionSeq, editorSeq int) string {
	return "Editor Decision " + strconv.Itoa(decisionSeq) + "  (Editor " + strconv.Itoa(editorSeq) + ")"
}

func editorDecisionDateColumn(decisionSeq, editorSeq int) string {
	return "Date decided " + strconv.Itoa(decisionSeq) + "  (Editor " + strconv.Itoa(editorSeq) + ")"
}
