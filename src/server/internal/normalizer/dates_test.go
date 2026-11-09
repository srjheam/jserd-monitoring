package normalizer

import (
	"testing"
	"time"
)

func TestParseDateReadsEveryOJSLayout(t *testing.T) {
	want := time.Date(2026, 5, 7, 10, 30, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name  string
		value string
		want  time.Time
	}{
		{"rfc3339", "2026-05-07T10:30:00Z", want},
		{"rfc3339 with offset", "2026-05-07T07:30:00-03:00", want},
		{"date and time", "2026-05-07 10:30:00", want},
		{"date only", "2026-05-07", time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := ParseDate(testCase.value)
			if !ok {
				t.Fatalf("ParseDate(%q) reported no match", testCase.value)
			}
			if !got.Equal(testCase.want) {
				t.Fatalf("ParseDate(%q) = %s, want %s", testCase.value, got, testCase.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("ParseDate(%q) returned %s, want UTC", testCase.value, got.Location())
			}
		})
	}
}

// A PocketBase timestamp is not an OJS date. Reading one here is what made the
// three former parsers drift apart, so ParseDate must refuse it and the caller
// must use record.GetDateTime instead.
func TestParseDateRejectsWhatIsNotAnOJSDate(t *testing.T) {
	for _, value := range []string{"", "   ", "not a date", "07/05/2026", "2026-05-07 10:30:00.000Z"} {
		if _, ok := ParseDate(value); ok {
			t.Fatalf("ParseDate(%q) reported a match, want none", value)
		}
	}
}
