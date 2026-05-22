package app

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type TemplateRenderError struct {
	Field string
	Err   error
}

func (e *TemplateRenderError) Error() string {
	return fmt.Sprintf("render %s: %v", e.Field, e.Err)
}

func (e *TemplateRenderError) Unwrap() error {
	return e.Err
}

type emailRenderer struct{}

func NewEmailRenderer() EmailRenderer {
	return emailRenderer{}
}

func (emailRenderer) Render(t ResolvedTemplate, vars map[string]any) (RenderedEmail, error) {
	for _, name := range t.RequiredVars {
		value, ok := vars[name]
		if !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return RenderedEmail{}, fmt.Errorf("missing required template variable: %s", name)
		}
	}
	subject, err := renderTemplate("subject", t.Subject, vars)
	if err != nil {
		return RenderedEmail{}, err
	}
	subject = strings.TrimRight(subject, "\r\n")
	textBody, err := renderTemplate("body.txt", t.TextBody, vars)
	if err != nil {
		return RenderedEmail{}, err
	}
	textBody = strings.TrimRight(textBody, "\r\n")
	if t.TrailingLF {
		textBody += "\n"
	}
	htmlBody := ""
	if strings.TrimSpace(t.HTMLBody) != "" {
		htmlBody, err = renderTemplate("body.html", t.HTMLBody, vars)
		if err != nil {
			return RenderedEmail{}, err
		}
	}
	return RenderedEmail{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}, nil
}

func renderTemplate(field, src string, vars map[string]any) (string, error) {
	tmpl, err := template.New(field).Parse(src)
	if err != nil {
		return "", &TemplateRenderError{Field: field, Err: err}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", &TemplateRenderError{Field: field, Err: err}
	}
	return buf.String(), nil
}
