package registry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationtemplates "github.com/cobo/cobo_iam_services/internal/notification/templates"
	yaml "go.yaml.in/yaml/v2"
)

type EmbedRegistry struct {
	fsys fs.FS
}

type metaFile struct {
	DefaultLocale string        `yaml:"default_locale"`
	Variables     []metaVarSpec `yaml:"variables"`
	TrailingLF    bool          `yaml:"trailing_lf"`
}

type metaVarSpec struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

func NewEmbedRegistry() *EmbedRegistry {
	return &EmbedRegistry{fsys: notificationtemplates.FS()}
}

func NewEmbedRegistryFS(fsys fs.FS) *EmbedRegistry {
	return &EmbedRegistry{fsys: fsys}
}

func (r *EmbedRegistry) Resolve(_ context.Context, key, locale string) (notificationapp.ResolvedTemplate, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return notificationapp.ResolvedTemplate{}, fmt.Errorf("template key is required")
	}
	metaBytes, err := fs.ReadFile(r.fsys, path.Join(key, "meta.yaml"))
	if err != nil {
		return notificationapp.ResolvedTemplate{}, fmt.Errorf("read meta for %s: %w", key, err)
	}
	var meta metaFile
	if err := yaml.Unmarshal(metaBytes, &meta); err != nil {
		return notificationapp.ResolvedTemplate{}, fmt.Errorf("decode meta for %s: %w", key, err)
	}
	resolvedLocale, err := r.resolveLocale(key, locale, meta.DefaultLocale)
	if err != nil {
		return notificationapp.ResolvedTemplate{}, err
	}
	subject, err := readOptional(r.fsys, path.Join(key, resolvedLocale, "subject.txt"))
	if err != nil {
		return notificationapp.ResolvedTemplate{}, err
	}
	textBody, err := readOptional(r.fsys, path.Join(key, resolvedLocale, "body.txt"))
	if err != nil {
		return notificationapp.ResolvedTemplate{}, err
	}
	htmlBody, err := readOptional(r.fsys, path.Join(key, resolvedLocale, "body.html"))
	if err != nil {
		return notificationapp.ResolvedTemplate{}, err
	}
	required := make([]string, 0, len(meta.Variables))
	for _, variable := range meta.Variables {
		if variable.Required {
			required = append(required, variable.Name)
		}
	}
	sort.Strings(required)
	return notificationapp.ResolvedTemplate{
		Key:          key,
		Locale:       resolvedLocale,
		Subject:      subject,
		TextBody:     textBody,
		HTMLBody:     htmlBody,
		RequiredVars: required,
		TrailingLF:   meta.TrailingLF,
	}, nil
}

func (r *EmbedRegistry) resolveLocale(key, requested, defaultLocale string) (string, error) {
	candidates := []string{strings.TrimSpace(requested), strings.TrimSpace(defaultLocale), "vi"}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := fs.Stat(r.fsys, path.Join(key, candidate)); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no locale found for template %s", key)
}

func readOptional(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err == nil {
		return string(b), nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return "", fmt.Errorf("read template file %s: %w", name, err)
}
