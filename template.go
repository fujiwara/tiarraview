package tiarraview

import (
	"embed"
	"html/template"
	"io"
	"log/slog"
	"net/url"

	"github.com/labstack/echo/v4"
)

//go:embed views/*.html
var viewFiles embed.FS

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	if m, ok := data.(map[string]interface{}); ok {
		m["Root"] = config.Server.Root
		data = m
	}
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		slog.Error("failed to render template", "error", err)
		return err
	}
	return nil
}

func newTemplates() *Template {
	funcMap := template.FuncMap{
		"urlEscape": url.QueryEscape,
	}
	t := template.New("").Funcs(funcMap)
	return &Template{
		templates: template.Must(t.ParseFS(viewFiles, "views/*.html")),
	}
}
