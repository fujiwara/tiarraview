package tiarraview

import (
	"embed"
	"io"
	"net/url"
	"text/template"

	"github.com/labstack/echo/v4"
)

//go:embed views/*.html
var viewFiles embed.FS

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
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
