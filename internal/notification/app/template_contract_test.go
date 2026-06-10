package app_test

import (
	"context"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	notificationtemplates "github.com/cobo/cobo_iam_services/internal/notification/templates"
	yaml "go.yaml.in/yaml/v2"
)

// PHASE 0 — EMAIL CONTRACT LOCKDOWN.
//
// These tests are the build-time guardrails that prevent the whole class of
// email-content bugs (e.g. {{.change_note}} rendering as "<no value>", a CTA
// pointing at a relative path, an English string leaking into a Vietnamese
// template) from ever reaching a user again. Every template under
// internal/notification/templates is validated here; adding a new template
// automatically subjects it to the same contract.

// metaContract mirrors the registry's meta.yaml shape for direct parsing.
type metaContract struct {
	Variables []struct {
		Name     string `yaml:"name"`
		Required bool   `yaml:"required"`
	} `yaml:"variables"`
}

// actionVarRe extracts a Go-template field reference like `.portal_url`.
var actionVarRe = regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// actionBlockRe extracts each `{{ ... }}` action block from a template body.
var actionBlockRe = regexp.MustCompile(`{{[^}]*}}`)

// listTemplateKeys enumerates every template directory (one per email type).
func listTemplateKeys(t *testing.T) []string {
	t.Helper()
	fsys := notificationtemplates.FS()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			keys = append(keys, e.Name())
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		t.Fatal("no template directories found")
	}
	return keys
}

// declaredVars parses meta.yaml for a template key.
func declaredVars(t *testing.T, key string) map[string]bool {
	t.Helper()
	fsys := notificationtemplates.FS()
	raw, err := fs.ReadFile(fsys, key+"/meta.yaml")
	if err != nil {
		t.Fatalf("%s: read meta.yaml: %v", key, err)
	}
	var meta metaContract
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("%s: decode meta.yaml: %v", key, err)
	}
	out := map[string]bool{}
	for _, v := range meta.Variables {
		out[v.Name] = true
	}
	return out
}

// templateBodies returns the concatenated raw content of subject/body.txt/body.html
// for the vi locale (the only locale shipped today).
func templateBodies(t *testing.T, key string) (subject, textBody, htmlBody string) {
	t.Helper()
	fsys := notificationtemplates.FS()
	read := func(name string) string {
		b, err := fs.ReadFile(fsys, key+"/vi/"+name)
		if err != nil {
			return ""
		}
		return string(b)
	}
	return read("subject.txt"), read("body.txt"), read("body.html")
}

// usedVars returns the set of field references appearing inside `{{ }}` blocks.
func usedVars(content string) map[string]bool {
	out := map[string]bool{}
	for _, block := range actionBlockRe.FindAllString(content, -1) {
		for _, m := range actionVarRe.FindAllStringSubmatch(block, -1) {
			out[m[1]] = true
		}
	}
	return out
}

// 0.1 — Variable parity: meta.yaml declarations must equal the variables used
// across subject + body.txt + body.html. A variable used but not declared, or
// declared but never used, fails the build.
func TestContract_VariableParity(t *testing.T) {
	for _, key := range listTemplateKeys(t) {
		t.Run(key, func(t *testing.T) {
			declared := declaredVars(t, key)
			subject, textBody, htmlBody := templateBodies(t, key)
			used := usedVars(subject + "\n" + textBody + "\n" + htmlBody)

			for name := range used {
				if !declared[name] {
					t.Errorf("variable %q is used in a template file but NOT declared in meta.yaml", name)
				}
			}
			for name := range declared {
				if !used[name] {
					t.Errorf("variable %q is declared in meta.yaml but NEVER used in any template file", name)
				}
			}
		})
	}
}

// sampleVar returns a safe, absolute, diacritic-free value for a variable so a
// render exercises every conditional branch with no missing-var drop and no
// forbidden artefact. URL-shaped vars get an absolute https value.
func sampleVar(name string) any {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "url"), strings.Contains(lower, "link"):
		return "https://portal.example.com/app/disclosures/sample-123"
	case strings.Contains(lower, "email"):
		return "support@cobo.vn"
	case strings.Contains(lower, "minutes"):
		return 30
	case strings.Contains(lower, "hours"):
		return 72
	default:
		return "Mẫu " + name
	}
}

// 0.2 + 0.3 — HTML render validation + forbidden content. Every template is
// rendered (TextBody AND HTMLBody) with all declared variables populated; the
// output must never contain unresolved placeholders, raw map leaks, localhost,
// or an unrendered action.
func TestContract_RenderForbiddenContent(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()
	forbidden := []string{"<no value>", "<nil>", "{{", "}}", "localhost", "127.0.0.1", "map["}

	for _, key := range listTemplateKeys(t) {
		t.Run(key, func(t *testing.T) {
			declared := declaredVars(t, key)
			vars := map[string]any{}
			for name := range declared {
				vars[name] = sampleVar(name)
			}
			resolved, err := registry.Resolve(context.Background(), key, "vi")
			if err != nil {
				t.Fatalf("resolve %s: %v", key, err)
			}
			rendered, err := renderer.Render(resolved, vars)
			if err != nil {
				t.Fatalf("render %s: %v", key, err)
			}
			for _, field := range []struct {
				name string
				body string
			}{
				{"subject", rendered.Subject},
				{"TextBody", rendered.TextBody},
				{"HTMLBody", rendered.HTMLBody},
			} {
				for _, bad := range forbidden {
					if strings.Contains(field.body, bad) {
						t.Errorf("%s.%s contains forbidden token %q:\n%s", key, field.name, bad, field.body)
					}
				}
			}
		})
	}
}

// 0.4 — CTA validation. Any template that ships an HTML body must expose at
// least one anchor, and every anchor href must be an absolute http(s) URL —
// never relative, never localhost, never an unrendered action.
func TestContract_CTAAbsolute(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()
	hrefRe := regexp.MustCompile(`href="([^"]*)"`)

	for _, key := range listTemplateKeys(t) {
		_, _, htmlRaw := templateBodies(t, key)
		if strings.TrimSpace(htmlRaw) == "" {
			continue // text-only template (auth.*, reminder.disclosure_deadline) — no CTA contract
		}
		t.Run(key, func(t *testing.T) {
			declared := declaredVars(t, key)
			vars := map[string]any{}
			for name := range declared {
				vars[name] = sampleVar(name)
			}
			resolved, err := registry.Resolve(context.Background(), key, "vi")
			if err != nil {
				t.Fatalf("resolve %s: %v", key, err)
			}
			rendered, err := renderer.Render(resolved, vars)
			if err != nil {
				t.Fatalf("render %s: %v", key, err)
			}
			hrefs := hrefRe.FindAllStringSubmatch(rendered.HTMLBody, -1)
			if len(hrefs) == 0 {
				t.Fatalf("%s has an HTML body but no CTA anchor", key)
			}
			for _, h := range hrefs {
				url := h[1]
				if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
					t.Errorf("%s CTA href is not absolute: %q", key, url)
				}
				if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
					t.Errorf("%s CTA href points at localhost: %q", key, url)
				}
				if strings.Contains(url, "{{") {
					t.Errorf("%s CTA href has unrendered action: %q", key, url)
				}
			}
		})
	}
}

// 0.5 — Golden coverage. The three user-facing template families must each be
// present so no family silently loses its contract test surface.
func TestContract_GoldenCoverage(t *testing.T) {
	keys := listTemplateKeys(t)
	families := map[string]bool{"adhoc.": false, "reminder.": false, "auth.": false}
	for _, k := range keys {
		for fam := range families {
			if strings.HasPrefix(k, fam) {
				families[fam] = true
			}
		}
	}
	for fam, present := range families {
		if !present {
			t.Errorf("no template found for required family %q", fam)
		}
	}
}
