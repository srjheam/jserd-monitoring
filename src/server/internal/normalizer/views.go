package normalizer

import (
	"io"
	"sort"
)

func NormalizeViews(r io.Reader, artifact ArtifactRef) ([]ViewMetric, error) {
	table, err := readCSV(r, artifact, []string{"Article ID", "Article Title", "Issue", "Date Published", "Abstract Views", "Total Galley Views"})
	if err != nil {
		return nil, err
	}

	galleyColumns := discoverGalleyColumns(table)
	metrics := make([]ViewMetric, 0, len(table.rows))
	for _, row := range table.rows {
		if metric, ok := normalizeMetric(table, row, galleyColumns); ok {
			metrics = append(metrics, metric)
		}
	}
	return metrics, nil
}

// viewProvenanceColumns maps each view metric field to the report column it
// comes from.
var viewProvenanceColumns = map[string]string{
	"articleId":        "Article ID",
	"title":            "Article Title",
	"issue":            "Issue",
	"datePublished":    "Date Published",
	"abstractViews":    "Abstract Views",
	"totalGalleyViews": "Total Galley Views",
}

// normalizeMetric reads one row of the Views report. A row that names no
// article id is not a view metric.
func normalizeMetric(table csvTable, row []string, galleyColumns []string) (ViewMetric, bool) {
	articleID := table.value(row, "Article ID")
	if articleID == "" {
		return ViewMetric{}, false
	}
	metric := ViewMetric{
		ArticleID:        articleID,
		Title:            table.value(row, "Article Title"),
		Issue:            table.value(row, "Issue"),
		DatePublished:    table.value(row, "Date Published"),
		AbstractViews:    parseInt(table.value(row, "Abstract Views")),
		TotalGalleyViews: parseInt(table.value(row, "Total Galley Views")),
		GalleyViews:      map[string]int{},
		Provenance:       map[string]FieldProvenance{},
	}
	for _, column := range galleyColumns {
		metric.GalleyViews[column] = parseInt(table.value(row, column))
	}
	addProvenance(table, row, metric.Provenance, viewProvenanceColumns)
	return metric, true
}

// discoverGalleyColumns lists the per-galley view columns, which the Views
// report places after "Total Galley Views", in the order the report holds them.
func discoverGalleyColumns(table csvTable) []string {
	totalIndex, ok := table.header["Total Galley Views"]
	if !ok {
		return nil
	}
	columns := make([]string, 0)
	for column, index := range table.header {
		if index > totalIndex {
			columns = append(columns, column)
		}
	}
	sort.Slice(columns, func(i, j int) bool { return table.header[columns[i]] < table.header[columns[j]] })
	return columns
}
