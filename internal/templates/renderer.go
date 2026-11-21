package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
)

type Page struct {
	Title string
}

type ErrorPage struct {
	Page
	StatusCode int
	Message    string
}

type Renderer struct {
	templates *template.Template
}

func Load(directoryPath string) (*Renderer, error) {
	functions := template.FuncMap{
		"documentTitle": documentTitle,
		"multiply":      func(left, right int64) int64 { return left * right },
	}

	parsedTemplates, err := template.New("pages").Funcs(functions).ParseGlob(filepath.Join(directoryPath, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Renderer{templates: parsedTemplates}, nil
}

func (renderer *Renderer) Render(responseWriter http.ResponseWriter, statusCode int, name string, view any) error {
	var rendered bytes.Buffer
	if err := renderer.templates.ExecuteTemplate(&rendered, name, view); err != nil {
		return fmt.Errorf("render %s template: %w", name, err)
	}

	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(statusCode)
	if _, err := rendered.WriteTo(responseWriter); err != nil {
		return fmt.Errorf("write %s template: %w", name, err)
	}
	return nil
}

func documentTitle(title string) string {
	if title == "BoardgameSort" {
		return title
	}
	return title + " - BoardgameSort"
}
