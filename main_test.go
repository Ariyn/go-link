package main

import (
	"testing"
)

func TestMatchParameterizedSlug(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		slug      string
		wantMatch bool
		wantParams map[string]string
	}{
		{
			name:      "single param",
			pattern:   "jira/{id}",
			slug:      "jira/PROJ-123",
			wantMatch: true,
			wantParams: map[string]string{"id": "PROJ-123"},
		},
		{
			name:      "multiple params",
			pattern:   "{org}/{repo}",
			slug:      "anthropic/claude",
			wantMatch: true,
			wantParams: map[string]string{"org": "anthropic", "repo": "claude"},
		},
		{
			name:      "param in middle",
			pattern:   "team/{name}/dashboard",
			slug:      "team/backend/dashboard",
			wantMatch: true,
			wantParams: map[string]string{"name": "backend"},
		},
		{
			name:      "no params exact match",
			pattern:   "docs/api",
			slug:      "docs/api",
			wantMatch: true,
			wantParams: map[string]string{},
		},
		{
			name:      "segment count mismatch",
			pattern:   "jira/{id}",
			slug:      "jira/PROJ-123/extra",
			wantMatch: false,
		},
		{
			name:      "static segment mismatch",
			pattern:   "jira/{id}",
			slug:      "confluence/PROJ-123",
			wantMatch: false,
		},
		{
			name:      "fewer segments",
			pattern:   "a/{id}/b",
			slug:      "a/1",
			wantMatch: false,
		},
		{
			name:      "single segment param",
			pattern:   "{id}",
			slug:      "hello",
			wantMatch: true,
			wantParams: map[string]string{"id": "hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, ok := matchParameterizedSlug(tt.pattern, tt.slug)
			if ok != tt.wantMatch {
				t.Fatalf("matchParameterizedSlug(%q, %q) matched=%v, want %v", tt.pattern, tt.slug, ok, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if len(params) != len(tt.wantParams) {
				t.Fatalf("got %d params, want %d", len(params), len(tt.wantParams))
			}
			for k, want := range tt.wantParams {
				if got := params[k]; got != want {
					t.Errorf("param[%q] = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestSubstituteParams(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		params map[string]string
		want   string
	}{
		{
			name:   "single substitution",
			url:    "https://jira.example.com/browse/{id}",
			params: map[string]string{"id": "PROJ-123"},
			want:   "https://jira.example.com/browse/PROJ-123",
		},
		{
			name:   "multiple substitutions",
			url:    "https://github.com/{org}/{repo}",
			params: map[string]string{"org": "anthropic", "repo": "claude"},
			want:   "https://github.com/anthropic/claude",
		},
		{
			name:   "same param used twice",
			url:    "https://example.com/{id}/details/{id}",
			params: map[string]string{"id": "42"},
			want:   "https://example.com/42/details/42",
		},
		{
			name:   "no placeholders",
			url:    "https://example.com/page",
			params: map[string]string{"id": "42"},
			want:   "https://example.com/page",
		},
		{
			name:   "param in query string",
			url:    "https://example.com/search?q={query}",
			params: map[string]string{"query": "hello"},
			want:   "https://example.com/search?q=hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteParams(tt.url, tt.params)
			if got != tt.want {
				t.Errorf("substituteParams(%q, %v) = %q, want %q", tt.url, tt.params, got, tt.want)
			}
		})
	}
}

func TestNormalizeSlug_WithParams(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantOk  bool
	}{
		{
			name:   "parameterized slug",
			raw:    "jira/{id}",
			want:   "jira/{id}",
			wantOk: true,
		},
		{
			name:   "multiple params",
			raw:    "{org}/{repo}",
			want:   "{org}/{repo}",
			wantOk: true,
		},
		{
			name:   "plain slug still works",
			raw:    "my-link",
			want:   "my-link",
			wantOk: true,
		},
		{
			name:   "uppercase normalized",
			raw:    "Jira/{ID}",
			want:   "jira/{id}",
			wantOk: true,
		},
		{
			name:   "leading trailing slashes trimmed",
			raw:    "/jira/{id}/",
			want:   "jira/{id}",
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeSlug(tt.raw)
			if ok != tt.wantOk {
				t.Fatalf("normalizeSlug(%q) ok=%v, want %v", tt.raw, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("normalizeSlug(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
