package normalizer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// submissionIDColumn is the report column that names a submission. Both the
// Articles report and the Reviews report carry it.
const submissionIDColumn = "Submission ID"

type csvTable struct {
	artifact ArtifactRef
	header   map[string]int
	rows     [][]string
}

func readCSV(r io.Reader, artifact ArtifactRef, required []string) (csvTable, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return csvTable{}, err
	}

	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(body), "\ufeff")))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return csvTable{}, err
	}
	if len(records) == 0 {
		return csvTable{}, fmt.Errorf("%s CSV has no header", artifact.Kind)
	}
	table := csvTable{
		artifact: artifact,
		header:   columnIndexes(records[0]),
		rows:     records[1:],
	}
	return table, missingColumnError(table, required, artifact.Kind)
}

// columnIndexes maps each column name of the header row to its position.
func columnIndexes(headers []string) map[string]int {
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}
	indexes := make(map[string]int, len(headers))
	for i, name := range headers {
		indexes[name] = i
	}
	return indexes
}

// missingColumnError names the first required column the report does not carry.
func missingColumnError(table csvTable, required []string, kind string) error {
	for _, column := range required {
		if !table.has(column) {
			return fmt.Errorf("%s CSV missing required column %q", kind, column)
		}
	}
	return nil
}

func (t csvTable) has(column string) bool {
	_, ok := t.header[column]
	return ok
}

func (t csvTable) value(row []string, column string) string {
	index, ok := t.header[column]
	if !ok || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func (t csvTable) provenance(row []string, column string) FieldProvenance {
	return FieldProvenance{
		Source:   t.artifact.Kind,
		Column:   column,
		Artifact: t.artifact.PathOrURL,
		Value:    t.value(row, column),
	}
}

// addProvenance records where each field came from. A column the report does
// not carry gets no entry.
func addProvenance(table csvTable, row []string, provenance map[string]FieldProvenance, columns map[string]string) {
	for key, column := range columns {
		if table.has(column) {
			provenance[key] = table.provenance(row, column)
		}
	}
}

func parseInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "1", "sim":
		return true
	default:
		return false
	}
}
