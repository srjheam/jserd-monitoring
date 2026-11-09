package normalizer

import (
	"io"
	"slices"
)

func NormalizeArticles(r io.Reader, artifact ArtifactRef) ([]Submission, error) {
	table, err := readCSV(r, artifact, []string{submissionIDColumn, "Title", "Abstract", "Status", "URL", "Date submitted", "Last modified"})
	if err != nil {
		return nil, err
	}

	layout := articleColumnLayout{
		authors:   discoverSequences(table.header, authorHeaderRE, 1),
		editors:   discoverSequences(table.header, editorHeaderRE, 1),
		decisions: discoverEditorDecisions(table.header),
	}
	submissions := make([]Submission, 0, len(table.rows))
	for _, row := range table.rows {
		if submission, ok := normalizeSubmission(table, row, layout); ok {
			submissions = append(submissions, submission)
		}
	}
	return submissions, nil
}

// articleProvenanceColumns maps each submission field to the report column it
// comes from.
var articleProvenanceColumns = map[string]string{
	"submissionId":  submissionIDColumn,
	"title":         "Title",
	"abstractText":  "Abstract",
	"status":        "Status",
	"url":           "URL",
	"doi":           "DOI",
	"dateSubmitted": "Date submitted",
	"lastModified":  "Last modified",
}

// normalizeSubmission reads one row of the Articles report. A row that names no
// submission id is not a submission.
func normalizeSubmission(table csvTable, row []string, layout articleColumnLayout) (Submission, bool) {
	submissionID := table.value(row, submissionIDColumn)
	if submissionID == "" {
		return Submission{}, false
	}
	submission := Submission{
		OJSSubmissionID: submissionID,
		Title:           table.value(row, "Title"),
		AbstractText:    table.value(row, "Abstract"),
		Status:          table.value(row, "Status"),
		URL:             table.value(row, "URL"),
		DOI:             table.value(row, "DOI"),
		DateSubmitted:   table.value(row, "Date submitted"),
		LastModified:    table.value(row, "Last modified"),
		Authors:         normalizeAuthors(table, row, layout.authors),
		Editors:         normalizeEditors(table, row, layout.editors, layout.decisions),
		Provenance:      map[string]FieldProvenance{},
	}
	addProvenance(table, row, submission.Provenance, articleProvenanceColumns)
	return submission, true
}

func normalizeAuthors(table csvTable, row []string, seqs []int) []Author {
	authors := make([]Author, 0, len(seqs))
	for _, seq := range seqs {
		author := Author{
			Sequence:     seq,
			GivenName:    table.value(row, authorColumn("Given Name", seq)),
			FamilyName:   table.value(row, authorColumn("Family Name", seq)),
			ORCID:        table.value(row, authorColumn("ORCID iD", seq)),
			Country:      table.value(row, authorColumn("Country", seq)),
			Affiliation:  table.value(row, authorColumn("Affiliation", seq)),
			Email:        table.value(row, authorColumn("Email", seq)),
			HomepageURL:  table.value(row, authorColumn("Homepage URL", seq)),
			BioStatement: table.value(row, authorColumn("Bio Statement (e.g., department and rank)", seq)),
		}
		if anyNonEmpty(author.GivenName, author.FamilyName, author.Email, author.ORCID, author.Affiliation) {
			authors = append(authors, author)
		}
	}
	return authors
}

func normalizeEditors(table csvTable, row []string, seqs []int, decisions map[int][]int) []Editor {
	editors := make([]Editor, 0, len(seqs))
	for _, seq := range seqs {
		editor := Editor{
			Sequence:   seq,
			GivenName:  table.value(row, editorColumn("Given Name", seq)),
			FamilyName: table.value(row, editorColumn("Family Name", seq)),
			ORCID:      table.value(row, editorColumn("ORCID iD", seq)),
			Email:      table.value(row, editorColumn("Email", seq)),
			Decisions:  normalizeEditorDecisions(table, row, seq, decisions[seq]),
		}
		if anyNonEmpty(editor.GivenName, editor.FamilyName, editor.Email) || len(editor.Decisions) > 0 {
			editors = append(editors, editor)
		}
	}
	return editors
}

// anyNonEmpty reports whether at least one value carries text.
func anyNonEmpty(values ...string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return value != "" })
}

func normalizeEditorDecisions(table csvTable, row []string, editorSeq int, decisionSeqs []int) []EditorDecision {
	decisions := make([]EditorDecision, 0, len(decisionSeqs))
	for _, seq := range decisionSeqs {
		decision := table.value(row, editorDecisionColumn(seq, editorSeq))
		date := table.value(row, editorDecisionDateColumn(seq, editorSeq))
		if decision == "" && date == "" {
			continue
		}
		decisions = append(decisions, EditorDecision{
			Sequence:    seq,
			Decision:    decision,
			DateDecided: date,
		})
	}
	return decisions
}
