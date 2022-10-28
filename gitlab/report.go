package gitlab

import (
	htmlTemplate "html/template"
	"os"
	"text/template"

	"github.com/obcode/glabs/config"
	r "github.com/obcode/glabs/gitlab/report"
)

func (c *Client) Report(assignmentCfg *config.AssignmentConfig, templateFile *string, output *string) {
	report := c.report(assignmentCfg)

	var (
		tmpl *template.Template
		err  error
	)
	if templateFile != nil {
		tmpl, err = template.ParseFiles(*templateFile)
	} else {
		tmpl, err = template.New("text").Parse(r.TextTemplate)
	}

	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(os.Stdout, report)
	if err != nil {
		panic(err)
	}
}

func (c *Client) ReportHTML(assignmentCfg *config.AssignmentConfig, templateFile *string, output *string) {
	report := c.report(assignmentCfg)

	var (
		tmpl *htmlTemplate.Template
		err  error
	)
	if templateFile != nil {
		tmpl, err = htmlTemplate.ParseFiles(*templateFile)
	} else {
		tmpl, err = htmlTemplate.New("html").Parse(r.HTMLTemplate)
	}
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(os.Stdout, report)
	if err != nil {
		panic(err)
	}
}
