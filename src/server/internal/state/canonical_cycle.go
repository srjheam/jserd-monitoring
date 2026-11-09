package state

import (
	"server/internal/audit"
	"server/internal/normalizer"
	"sort"
)

func canonicalCycle(input normalizer.NormalizedCycle) normalizer.NormalizedCycle {
	cycle := input
	cycle.Submissions = append([]normalizer.Submission(nil), input.Submissions...)
	for submissionIdx := range cycle.Submissions {
		cycle.Submissions[submissionIdx].Authors = append([]normalizer.Author(nil), cycle.Submissions[submissionIdx].Authors...)
		cycle.Submissions[submissionIdx].Editors = append([]normalizer.Editor(nil), cycle.Submissions[submissionIdx].Editors...)
		sort.SliceStable(cycle.Submissions[submissionIdx].Authors, func(a, b int) bool {
			return cycle.Submissions[submissionIdx].Authors[a].Sequence < cycle.Submissions[submissionIdx].Authors[b].Sequence
		})
		for j := range cycle.Submissions[submissionIdx].Editors {
			cycle.Submissions[submissionIdx].Editors[j].Decisions = append([]normalizer.EditorDecision(nil), cycle.Submissions[submissionIdx].Editors[j].Decisions...)
			sort.SliceStable(cycle.Submissions[submissionIdx].Editors[j].Decisions, func(a, b int) bool {
				return cycle.Submissions[submissionIdx].Editors[j].Decisions[a].Sequence < cycle.Submissions[submissionIdx].Editors[j].Decisions[b].Sequence
			})
		}
		sort.SliceStable(cycle.Submissions[submissionIdx].Editors, func(a, b int) bool {
			return cycle.Submissions[submissionIdx].Editors[a].Sequence < cycle.Submissions[submissionIdx].Editors[b].Sequence
		})
	}
	sort.SliceStable(cycle.Submissions, func(i, j int) bool {
		return cycle.Submissions[i].OJSSubmissionID < cycle.Submissions[j].OJSSubmissionID
	})
	cycle.Reviews = append([]normalizer.ReviewAssignment(nil), input.Reviews...)
	sort.SliceStable(cycle.Reviews, func(i, j int) bool { return cycle.Reviews[i].Key < cycle.Reviews[j].Key })
	cycle.ViewMetrics = append([]normalizer.ViewMetric(nil), input.ViewMetrics...)
	sort.SliceStable(cycle.ViewMetrics, func(i, j int) bool { return cycle.ViewMetrics[i].ArticleID < cycle.ViewMetrics[j].ArticleID })
	cycle.Artifacts = append([]audit.RawArtifact(nil), input.Artifacts...)
	sort.SliceStable(cycle.Artifacts, func(i, j int) bool {
		if cycle.Artifacts[i].Kind == cycle.Artifacts[j].Kind {
			return cycle.Artifacts[i].PathOrURL < cycle.Artifacts[j].PathOrURL
		}
		return cycle.Artifacts[i].Kind < cycle.Artifacts[j].Kind
	})
	return cycle
}
