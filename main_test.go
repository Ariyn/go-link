package main

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func makeLinkRecord() *core.Record {
	collection := core.NewCollection(core.CollectionTypeBase, "links")
	collection.Fields.Add(
		&core.TextField{Name: "slug", Required: true},
		&core.URLField{Name: "target_url", Required: true},
		&core.BoolField{Name: "enabled"},
		&core.DateField{Name: "expires_at"},
		&core.NumberField{Name: "ttl_seconds"},
	)

	record := core.NewRecord(collection)
	record.Set("slug", "demo")
	record.Set("target_url", "https://example.com")
	record.Set("enabled", true)
	return record
}

func mustParseDateTime(t *testing.T, value string) types.DateTime {
	t.Helper()

	d, err := types.ParseDateTime(value)
	if err != nil {
		t.Fatalf("ParseDateTime(%q) failed: %v", value, err)
	}

	return d
}

func TestMatchParameterizedSlug(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		slug       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{
			name:       "single param",
			pattern:    "jira/{id}",
			slug:       "jira/PROJ-123",
			wantMatch:  true,
			wantParams: map[string]string{"id": "PROJ-123"},
		},
		{
			name:       "multiple params",
			pattern:    "{org}/{repo}",
			slug:       "anthropic/claude",
			wantMatch:  true,
			wantParams: map[string]string{"org": "anthropic", "repo": "claude"},
		},
		{
			name:       "param in middle",
			pattern:    "team/{name}/dashboard",
			slug:       "team/backend/dashboard",
			wantMatch:  true,
			wantParams: map[string]string{"name": "backend"},
		},
		{
			name:       "no params exact match",
			pattern:    "docs/api",
			slug:       "docs/api",
			wantMatch:  true,
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
			name:       "single segment param",
			pattern:    "{id}",
			slug:       "hello",
			wantMatch:  true,
			wantParams: map[string]string{"id": "hello"},
		},
		{
			name:       "korean value in param",
			pattern:    "wiki/{name}",
			slug:       "wiki/한글",
			wantMatch:  true,
			wantParams: map[string]string{"name": "한글"},
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
		name   string
		raw    string
		want   string
		wantOk bool
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
		{
			name:   "korean slug",
			raw:    "wiki/한글",
			want:   "wiki/한글",
			wantOk: true,
		},
		{
			name:   "korean parameterized slug",
			raw:    "위키/{id}",
			want:   "위키/{id}",
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

func TestNormalizeTTLAndExpiry(t *testing.T) {
	now := mustParseDateTime(t, "2026-03-19 00:00:00.000Z")

	t.Run("ttl populates expires_at", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("ttl_seconds", 3600)

		if err := normalizeTTLAndExpiry(record, now); err != nil {
			t.Fatalf("normalizeTTLAndExpiry() error = %v", err)
		}

		expiresAt := record.GetDateTime("expires_at")
		if expiresAt.IsZero() {
			t.Fatal("expires_at should be set from ttl_seconds")
		}

		want := now.Add(1 * time.Hour)
		if !expiresAt.Equal(want) {
			t.Fatalf("expires_at = %q, want %q", expiresAt.String(), want.String())
		}
	})

	t.Run("expires_at wins over ttl_seconds", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("ttl_seconds", 3600)
		record.Set("expires_at", "2026-03-20 00:00:00.000Z")

		if err := normalizeTTLAndExpiry(record, now); err != nil {
			t.Fatalf("normalizeTTLAndExpiry() error = %v", err)
		}

		expiresAt := record.GetDateTime("expires_at")
		want := mustParseDateTime(t, "2026-03-20 00:00:00.000Z")
		if !expiresAt.Equal(want) {
			t.Fatalf("expires_at = %q, want %q", expiresAt.String(), want.String())
		}
	})

	t.Run("ttl_seconds must be positive", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("ttl_seconds", 0)

		if err := normalizeTTLAndExpiry(record, now); err == nil {
			t.Fatal("expected error for ttl_seconds <= 0")
		}
	})
}

func TestIsActiveRecord(t *testing.T) {
	now := mustParseDateTime(t, "2026-03-19 00:00:00.000Z")

	t.Run("enabled without expires_at is active", func(t *testing.T) {
		record := makeLinkRecord()
		if !isActiveRecord(record, now) {
			t.Fatal("expected active record")
		}
	})

	t.Run("disabled is inactive", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("enabled", false)
		if isActiveRecord(record, now) {
			t.Fatal("expected disabled record to be inactive")
		}
	})

	t.Run("expired is inactive", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("expires_at", "2026-03-18 23:59:59.000Z")
		if isActiveRecord(record, now) {
			t.Fatal("expected expired record to be inactive")
		}
	})

	t.Run("future expires_at is active", func(t *testing.T) {
		record := makeLinkRecord()
		record.Set("expires_at", "2026-03-19 01:00:00.000Z")
		if !isActiveRecord(record, now) {
			t.Fatal("expected non-expired record to be active")
		}
	})
}
