package gitlab

import (
	"encoding/json"
	"fmt"
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

	if output != nil {
		os.Remove(*output)
		f, err := os.Create(*output)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = tmpl.Execute(f, report)
		if err != nil {
			panic(err)
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			panic(err)
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
		panic(err)
	}

	if output != nil {
		os.Remove(*output)
		f, err := os.Create(*output)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = tmpl.Execute(f, report)
		if err != nil {
			panic(err)
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			panic(err)
		}
	}
}

func (c *Client) ReportJSON(assignmentCfg *config.AssignmentConfig, output *string) {
	report := c.report(assignmentCfg)

	json, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}

	if output != nil {
		err := os.WriteFile(*output, json, 0644)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println(string(json))
	}
}
