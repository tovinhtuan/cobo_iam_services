package app

import "context"

type TemplateRegistry interface {
	Resolve(ctx context.Context, key, locale string) (ResolvedTemplate, error)
}

type EmailRenderer interface {
	Render(t ResolvedTemplate, vars map[string]any) (RenderedEmail, error)
}

type ResolvedTemplate struct {
	Key          string
	Locale       string
	Subject      string
	TextBody     string
	HTMLBody     string
	RequiredVars []string
	TrailingLF   bool
}

type RenderedEmail struct {
	Subject  string
	TextBody string
	HTMLBody string
}
