package gitlab

import (
	"encoding/json"
	"fmt"
	htmlTemplate "html/template"
	"os"
	"text/template"

	"github.com/obcode/glabs/v3/config"
	r "github.com/obcode/glabs/v3/gitlab/report"
)

// ReportData returns the structured report for an assignment without rendering it
// to text/HTML. It is the data-returning entry point the web server uses; the
// CLI's Report/ReportHTML render this same data.
func (c *Client) ReportData(assignmentCfg *config.AssignmentConfig) (*r.Reports, error) {
	return c.report(assignmentCfg)
}

func (c *Client) Report(assignmentCfg *config.AssignmentConfig, templateFile *string, output *string) error {
	report, err := c.report(assignmentCfg)
	if err != nil {
		return err
	}

	var tmpl *template.Template
	if templateFile != nil {
		tmpl, err = template.ParseFiles(*templateFile)
	} else {
		tmpl, err = template.New("text").Parse(r.TextTemplate)
	}

	if err != nil {
		return err
	}

	if output != nil {
		os.Remove(*output) //nolint
		f, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer f.Close() //nolint
		err = tmpl.Execute(f, report)
		if err != nil {
			return err
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ReportHTML(assignmentCfg *config.AssignmentConfig, templateFile *string, output *string) error {
	report, err := c.report(assignmentCfg)
	if err != nil {
		return err
	}

	var tmpl *htmlTemplate.Template
	if templateFile != nil {
		tmpl, err = htmlTemplate.ParseFiles(*templateFile)
	} else {
		tmpl, err = htmlTemplate.New("html").Parse(r.HTMLTemplate)
	}
	if err != nil {
		return err
	}

	if output != nil {
		os.Remove(*output) //nolint
		f, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer f.Close() //nolint
		err = tmpl.Execute(f, report)
		if err != nil {
			return err
		}
	} else {
		err = tmpl.Execute(os.Stdout, report)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ReportJSON(assignmentCfg *config.AssignmentConfig, output *string) error {
	report, err := c.report(assignmentCfg)
	if err != nil {
		return err
	}

	json, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if output != nil {
		err := os.WriteFile(*output, json, 0644)
		if err != nil {
			return err
		}
	} else {
		fmt.Println(string(json))
	}
	return nil
}
