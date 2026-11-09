package pbfilter_test

import (
	"server/internal/pbfilter"
	"testing"
)

func TestIDFilterOnEmptyIDsProducesNoFilter(t *testing.T) {
	expr, params := pbfilter.IDFilter("id", nil)
	if expr != "" || len(params) != 0 {
		t.Fatalf("expr=%q params=%#v, want an empty filter", expr, params)
	}
}

func TestIDFilterOnOneID(t *testing.T) {
	expr, params := pbfilter.IDFilter("id", []string{"a"})
	if expr != "id = {:idf0}" {
		t.Fatalf("expr=%q", expr)
	}
	if params["idf0"] != "a" {
		t.Fatalf("params=%#v", params)
	}
}

func TestIDFilterJoinsMultipleIDsWithOr(t *testing.T) {
	expr, params := pbfilter.IDFilter("id", []string{"a", "b", "c"})
	want := "id = {:idf0} || id = {:idf1} || id = {:idf2}"
	if expr != want {
		t.Fatalf("expr=%q, want %q", expr, want)
	}
	for i, id := range []string{"a", "b", "c"} {
		key := []string{"idf0", "idf1", "idf2"}[i]
		if params[key] != id {
			t.Fatalf("params[%s]=%v, want %q", key, params[key], id)
		}
	}
}
