package query

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseJobsListDefaultLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/internal/jobs", nil)

	parsed, err := ParseJobsList(req)
	if err != nil {
		t.Fatalf("ParseJobsList() error = %v", err)
	}

	if parsed.Limit != DefaultListLimit {
		t.Errorf("limit = %d, want %d", parsed.Limit, DefaultListLimit)
	}

	if parsed.OpenJobsOnly {
		t.Error("OpenJobsOnly = true, want false")
	}
}

func TestParseJobsListRejectsLimitAboveMax(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/internal/jobs?limit=501", nil)

	_, err := ParseJobsList(req)
	if err == nil {
		t.Fatal("ParseJobsList() expected error")
	}
}

func TestParseJobsListOpenJobsOnly(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    bool
		wantErr bool
	}{
		{name: "true", query: "open_jobs_only=true", want: true},
		{name: "false", query: "open_jobs_only=false"},
		{name: "with limit", query: "limit=2&open_jobs_only=true", want: true},
		{name: "not a boolean", query: "open_jobs_only=yes", wantErr: true},
		{name: "numeric", query: "open_jobs_only=1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/internal/jobs?"+tt.query, nil)

			parsed, err := ParseJobsList(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseJobsList() expected error")
				}

				if !strings.Contains(err.Error(), "open_jobs_only") {
					t.Fatalf("error = %q, want open_jobs_only", err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseJobsList() error = %v", err)
			}

			if parsed.OpenJobsOnly != tt.want {
				t.Errorf("OpenJobsOnly = %v, want %v", parsed.OpenJobsOnly, tt.want)
			}

			if tt.query == "limit=2&open_jobs_only=true" && parsed.Limit != 2 {
				t.Errorf("limit = %d, want 2", parsed.Limit)
			}
		})
	}
}
