package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/spf13/cobra"
)

var slugPattern = regexp.MustCompile("^[a-z0-9_/{}-]+$")
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
	_, err := app.FindCollectionByNameOrId("links")
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	collection := core.NewCollection(core.CollectionTypeBase, "links")
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
	)
	collection.AddIndex("idx_links_slug", true, "`slug`", "")

	return app.Save(collection)
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

			return e.Next()
		},
	})

	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			e.Router.GET("/{slug...}", func(re *core.RequestEvent) error {
				slug, ok := normalizeSlug(re.Request.PathValue("slug"))
				if !ok {
					return re.NoContent(http.StatusNotFound)
				}

				record, err := app.FindFirstRecordByFilter(
					"links",
					"slug = {:slug} && enabled = true",
					dbx.Params{"slug": slug},
				)

				var targetURL string
				if err != nil {
					// No exact match — try parameterized slugs
					records, findErr := app.FindAllRecords(
						"links",
						dbx.NewExp("slug LIKE '%{%}%' AND enabled = {:enabled}", dbx.Params{"enabled": true}),
					)
					if findErr != nil || len(records) == 0 {
						return re.NoContent(http.StatusNotFound)
					}

					var matched bool
					for _, r := range records {
						params, ok := matchParameterizedSlug(r.GetString("slug"), slug)
						if ok {
							record = r
							raw := substituteParams(r.GetString("target_url"), params)
							targetURL, ok = normalizeTargetURL(raw)
							if !ok {
								return re.NoContent(http.StatusNotFound)
							}
							matched = true
							break
						}
					}
					if !matched {
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
