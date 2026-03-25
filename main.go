package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cobra"
)

var slugPattern = regexp.MustCompile(`^[\p{L}\p{N}_/{}-]+$`)
var paramPattern = regexp.MustCompile(`\{([^}]+)\}`)

func normalizeSlug(raw string) (string, bool) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	slug = strings.Trim(slug, "/")
	if slug == "" || !slugPattern.MatchString(slug) {
		return "", false
	}
	return slug, true
}

func normalizeTargetURL(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), true
	default:
		return "", false
	}
}

// matchParameterizedSlug checks if the incoming slug matches a parameterized
// slug pattern (e.g., "jira/{id}") and returns extracted parameter values.
func matchParameterizedSlug(pattern, slug string) (map[string]string, bool) {
	patternParts := strings.Split(pattern, "/")
	slugParts := strings.Split(slug, "/")

	if len(patternParts) != len(slugParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		if paramPattern.MatchString(part) {
			name := paramPattern.FindStringSubmatch(part)[1]
			params[name] = slugParts[i]
		} else if part != slugParts[i] {
			return nil, false
		}
	}

	return params, true
}

// substituteParams replaces {name} placeholders in a URL with actual values.
func substituteParams(targetURL string, params map[string]string) string {
	result := targetURL
	for name, value := range params {
		result = strings.ReplaceAll(result, "{"+name+"}", value)
	}
	return result
}

func ensureLinksCollection(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("links")
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		collection = core.NewCollection(core.CollectionTypeBase, "links")
		collection.Fields.Add(
			&core.TextField{
				Name:     "slug",
				Required: true,
			},
			&core.URLField{
				Name:     "target_url",
				Required: true,
			},
			&core.BoolField{
				Name: "enabled",
			},
			&core.NumberField{
				Name: "hits",
			},
			&core.DateField{
				Name: "last_hit_at",
			},
			&core.DateField{
				Name: "expires_at",
			},
			&core.NumberField{
				Name: "ttl_seconds",
			},
		)
		collection.AddIndex("idx_links_slug", false, "`slug`", "")
		collection.AddIndex("idx_links_slug_enabled", false, "`slug`, `enabled`", "")

		return app.Save(collection)
	}

	changed := false

	if collection.Fields.GetByName("expires_at") == nil {
		collection.Fields.Add(&core.DateField{Name: "expires_at"})
		changed = true
	}

	if collection.Fields.GetByName("ttl_seconds") == nil {
		collection.Fields.Add(&core.NumberField{Name: "ttl_seconds"})
		changed = true
	}

	if idx := collection.GetIndex("idx_links_slug"); idx == "" || strings.Contains(strings.ToUpper(idx), "UNIQUE") {
		collection.AddIndex("idx_links_slug", false, "`slug`", "")
		changed = true
	}

	if collection.GetIndex("idx_links_slug_enabled") == "" {
		collection.AddIndex("idx_links_slug_enabled", false, "`slug`, `enabled`", "")
		changed = true
	}

	if !changed {
		return nil
	}

	return app.Save(collection)
}

func hasRawValue(v any) bool {
	if v == nil {
		return false
	}

	return strings.TrimSpace(fmt.Sprint(v)) != ""
}

func isExpiredRecord(record *core.Record, now types.DateTime) bool {
	expiresAt := record.GetDateTime("expires_at")
	if expiresAt.IsZero() {
		return false
	}

	return !expiresAt.After(now)
}

func isActiveRecord(record *core.Record, now types.DateTime) bool {
	if !record.GetBool("enabled") {
		return false
	}

	return !isExpiredRecord(record, now)
}

func isRecordNewer(candidate, selected *core.Record) bool {
	if selected == nil {
		return true
	}

	candidateUpdated := candidate.GetDateTime("updated")
	selectedUpdated := selected.GetDateTime("updated")
	if !candidateUpdated.Equal(selectedUpdated) {
		return candidateUpdated.After(selectedUpdated)
	}

	candidateCreated := candidate.GetDateTime("created")
	selectedCreated := selected.GetDateTime("created")
	if !candidateCreated.Equal(selectedCreated) {
		return candidateCreated.After(selectedCreated)
	}

	return candidate.Id > selected.Id
}

func normalizeTTLAndExpiry(record *core.Record, now types.DateTime) error {
	rawTTL := record.GetRaw("ttl_seconds")
	hasTTL := hasRawValue(rawTTL)
	hasExpiresAt := hasRawValue(record.GetRaw("expires_at"))

	if hasTTL {
		ttl := record.GetInt("ttl_seconds")
		if ttl <= 0 {
			return validation.Errors{
				"ttl_seconds": validation.NewError("validation_ttl_positive", "must be greater than 0"),
			}
		}

		if !hasExpiresAt {
			record.Set("expires_at", now.Add(time.Duration(ttl)*time.Second))
		}
	}

	if hasRawValue(record.GetRaw("expires_at")) {
		expiresAt := record.GetDateTime("expires_at")
		if expiresAt.IsZero() {
			return validation.Errors{
				"expires_at": validation.NewError("validation_expires_at_invalid", "must be a valid datetime"),
			}
		}
	}

	return nil
}

func ensureNoOtherActiveSlug(app core.App, record *core.Record, now types.DateTime) error {
	slug := record.GetString("slug")
	if slug == "" || !record.GetBool("enabled") {
		return nil
	}

	if isExpiredRecord(record, now) {
		return nil
	}

	records, err := app.FindAllRecords(
		"links",
		dbx.NewExp(
			"slug = {:slug} AND enabled = true AND (expires_at = '' OR expires_at > {:now}) AND id != {:id}",
			dbx.Params{"slug": slug, "now": now.String(), "id": record.Id},
		),
	)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		return nil
	}

	return validation.Errors{
		"slug": validation.NewError("validation_slug_active_exists", "an active link with this slug already exists"),
	}
}

func ensureHttp80Default(args []string) []string {
	if len(args) < 2 || args[1] != "serve" {
		return args
	}

	if hasFlag(args, "--http") {
		return args
	}

	return append(args, "--http=0.0.0.0:80")
}

func hasFlag(args []string, name string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			return true
		}
		if strings.HasPrefix(args[i], name+"=") {
			return true
		}
	}

	return false
}

func addCustomCLIHelp(app *pocketbase.PocketBase) {
	var serveCmd *cobra.Command
	for _, cmd := range app.RootCmd.Commands() {
		if cmd.Name() == "serve" {
			serveCmd = cmd
			break
		}
	}

	if serveCmd != nil {
		customNote := "Custom behavior:\n  - If --http is not set, defaults to 0.0.0.0:80.\n"
		serveCmd.Long = strings.TrimSpace(serveCmd.Long)
		if serveCmd.Long != "" {
			serveCmd.Long = serveCmd.Long + "\n\n" + customNote
		} else {
			serveCmd.Long = customNote
		}
	}

	rootTemplate := app.RootCmd.HelpTemplate()
	if !strings.Contains(rootTemplate, "Custom commands") {
		app.RootCmd.SetHelpTemplate(rootTemplate + "\nCustom commands:\n  serve\n    - Defaults to 0.0.0.0:80 when --http is omitted.\n")
	}
}

func main() {
	app := pocketbase.New()

	os.Args = ensureHttp80Default(os.Args)
	addCustomCLIHelp(app)

	app.OnBootstrap().Bind(&hook.Handler[*core.BootstrapEvent]{
		Func: func(e *core.BootstrapEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if err := ensureLinksCollection(e.App); err != nil {
				return err
			}

			return nil
		},
	})

	app.OnRecordCreate("links").Bind(&hook.Handler[*core.RecordEvent]{
		Func: func(e *core.RecordEvent) error {
			if e.Record.GetRaw("enabled") == nil {
				e.Record.Set("enabled", true)
			}

			now := types.NowDateTime().Add(0)
			if err := normalizeTTLAndExpiry(e.Record, now); err != nil {
				return err
			}

			if err := ensureNoOtherActiveSlug(e.App, e.Record, now); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordUpdate("links").Bind(&hook.Handler[*core.RecordEvent]{
		Func: func(e *core.RecordEvent) error {
			now := types.NowDateTime().Add(0)
			if err := normalizeTTLAndExpiry(e.Record, now); err != nil {
				return err
			}

			if err := ensureNoOtherActiveSlug(e.App, e.Record, now); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			e.Router.GET("/{slug...}", func(re *core.RequestEvent) error {
				now := types.NowDateTime().Add(0)
				slug, ok := normalizeSlug(re.Request.PathValue("slug"))
				if !ok {
					return re.NoContent(http.StatusNotFound)
				}

				exactRecords, err := app.FindAllRecords(
					"links",
					dbx.NewExp("slug = {:slug} AND enabled = true", dbx.Params{"slug": slug}),
				)
				if err != nil {
					return re.NoContent(http.StatusNotFound)
				}

				var record *core.Record
				exactExpired := false
				for _, r := range exactRecords {
					if isActiveRecord(r, now) {
						if isRecordNewer(r, record) {
							record = r
						}
						continue
					}

					if isExpiredRecord(r, now) {
						exactExpired = true
					}
				}

				var targetURL string
				if record == nil {
					// No exact match — try parameterized slugs
					records, findErr := app.FindAllRecords(
						"links",
						dbx.NewExp("slug LIKE '%{%}%' AND enabled = {:enabled}", dbx.Params{"enabled": true}),
					)
					if findErr != nil || len(records) == 0 {
						if exactExpired {
							return re.NoContent(http.StatusGone)
						}
						return re.NoContent(http.StatusNotFound)
					}

					var matchedExpired bool
					var matchedActive *core.Record
					for _, r := range records {
						params, ok := matchParameterizedSlug(r.GetString("slug"), slug)
						if !ok {
							continue
						}

						if isActiveRecord(r, now) {
							raw := substituteParams(r.GetString("target_url"), params)
							resolvedTargetURL, targetOK := normalizeTargetURL(raw)
							if !targetOK {
								continue
							}

							if isRecordNewer(r, matchedActive) {
								matchedActive = r
								targetURL = resolvedTargetURL
							}
							continue
						}

						if isExpiredRecord(r, now) {
							matchedExpired = true
						}
					}

					if matchedActive != nil {
						record = matchedActive
					} else if exactExpired || matchedExpired {
						return re.NoContent(http.StatusGone)
					} else {
						return re.NoContent(http.StatusNotFound)
					}
				} else {
					raw := record.GetString("target_url")
					var valid bool
					targetURL, valid = normalizeTargetURL(raw)
					if !valid {
						return re.NoContent(http.StatusNotFound)
					}
				}

				re.Response.Header().Set("Cache-Control", "no-store")

				record.Set("hits", record.GetInt("hits")+1)
				record.Set("last_hit_at", time.Now().UTC())
				if err := app.Save(record); err != nil {
					log.Printf("redirect stat update failed slug=%s error=%v", slug, err)
				}

				log.Printf("redirect slug=%s target_url=%s ip=%s", slug, targetURL, re.RealIP())
				return re.Redirect(http.StatusFound, targetURL)
			})

			return e.Next()
		},
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
