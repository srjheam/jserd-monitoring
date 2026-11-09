package editorial

import (
	"context"
	"errors"
	"server/internal/ojs"
	"testing"
)

type fakeSubmissionDetailSource struct {
	loginCalls int
	loginErr   error
	fetched    []string
	fetchErr   error
	details    map[string]ojs.SubmissionDetail
}

func (f *fakeSubmissionDetailSource) Login(context.Context, string, string) error {
	f.loginCalls++
	return f.loginErr
}

func (f *fakeSubmissionDetailSource) SubmissionDetail(_ context.Context, submissionID string) (ojs.SubmissionDetail, error) {
	f.fetched = append(f.fetched, submissionID)
	if f.fetchErr != nil {
		return ojs.SubmissionDetail{}, f.fetchErr
	}
	return f.details[submissionID], nil
}

func TestLoaderFetchesEachSubmissionOnlyOnceAcrossRepeatedGets(t *testing.T) {
	source := &fakeSubmissionDetailSource{details: map[string]ojs.SubmissionDetail{"1": {}}}
	loader := NewSubmissionDetailLoader(source, "u", "p")
	if _, err := loader.Get(context.Background(), "1"); err != nil {
		t.Fatalf("first get: %v", err)
	}
	if _, err := loader.Get(context.Background(), "1"); err != nil {
		t.Fatalf("second get: %v", err)
	}
	if len(source.fetched) != 1 {
		t.Fatalf("fetched=%v, want exactly one fetch", source.fetched)
	}
}

func TestLoaderLogsInOnceAcrossMultipleSubmissions(t *testing.T) {
	source := &fakeSubmissionDetailSource{details: map[string]ojs.SubmissionDetail{"1": {}, "2": {}}}
	loader := NewSubmissionDetailLoader(source, "u", "p")
	if _, err := loader.Get(context.Background(), "1"); err != nil {
		t.Fatalf("get 1: %v", err)
	}
	if _, err := loader.Get(context.Background(), "2"); err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if source.loginCalls != 1 {
		t.Fatalf("loginCalls=%d, want 1", source.loginCalls)
	}
}

func TestLoaderSkipsLoginWhenNeverCalled(t *testing.T) {
	source := &fakeSubmissionDetailSource{}
	NewSubmissionDetailLoader(source, "u", "p")
	if source.loginCalls != 0 {
		t.Fatalf("loginCalls=%d, want 0", source.loginCalls)
	}
}

func TestLoaderPropagatesLoginFailure(t *testing.T) {
	source := &fakeSubmissionDetailSource{loginErr: errors.New("login failed")}
	loader := NewSubmissionDetailLoader(source, "u", "p")
	if _, err := loader.Get(context.Background(), "1"); err == nil {
		t.Fatal("expected login failure to propagate")
	}
}

func TestLoaderPropagatesFetchFailureAndDoesNotCacheIt(t *testing.T) {
	source := &fakeSubmissionDetailSource{fetchErr: errors.New("fetch failed")}
	loader := NewSubmissionDetailLoader(source, "u", "p")
	if _, err := loader.Get(context.Background(), "1"); err == nil {
		t.Fatal("expected fetch failure to propagate")
	}
	source.fetchErr = nil
	source.details = map[string]ojs.SubmissionDetail{"1": {}}
	if _, err := loader.Get(context.Background(), "1"); err != nil {
		t.Fatalf("retry after fix: %v", err)
	}
	if len(source.fetched) != 2 {
		t.Fatalf("fetched=%v, want two attempts (failure was not cached)", source.fetched)
	}
}
