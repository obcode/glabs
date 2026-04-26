package gitlab

import (
	"encoding/json"
	"fmt"
	htmlTemplate "html/template"
	"os"
	"text/template"

	"github.com/obcode/glabs/v2/config"
	r "github.com/obcode/glabs/v2/gitlab/report"
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
		panicFunc(err)
	}

	if output != nil {
		os.Remove(*output) //nolint
		f, err := os.Create(*output)
		if err != nil {
			panicFunc(err)
		}
		defer f.Close() //nolint
		err = tmpl.Execute(f, report)
		if err != nil {
			panicFunc(err)
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			panicFunc(err)
		}
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
		panicFunc(err)
	}

	if output != nil {
		os.Remove(*output) //nolint
		f, err := os.Create(*output)
		if err != nil {
			panicFunc(err)
		}
		defer f.Close() //nolint
		err = tmpl.Execute(f, report)
		if err != nil {
			panicFunc(err)
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			panicFunc(err)
		}
	}
}

func (c *Client) ReportJSON(assignmentCfg *config.AssignmentConfig, output *string) {
	report := c.report(assignmentCfg)

	json, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panicFunc(err)
	}

	if output != nil {
		err := os.WriteFile(*output, json, 0644)
		if err != nil {
			panicFunc(err)
		}
	} else {
		fmt.Println(string(json))
	}
}
