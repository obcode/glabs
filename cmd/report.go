package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/obcode/glabs/config"
	"github.com/obcode/glabs/gitlab"
	"github.com/obcode/glabs/gitlab/report"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().BoolVar(&Html, "html", false, "generate HTML")
	reportCmd.Flags().BoolVar(&Json, "json", false, "generate JSON")
	reportCmd.Flags().StringVarP(&Template, "tmpl", "t", "", "use template for HTML")
	reportCmd.Flags().BoolVarP(&ExportTemplate, "export-default-template", "e", false, "export the default HTML template")
	reportCmd.Flags().StringVarP(&OutPut, "output", "o", "", "output to <file>")
}

var (
	reportCmd = &cobra.Command{
		Use:   "report course assignment",
		Short: "generate activity report",
		Long:  `Generate activity report`,
		Args: func(cmd *cobra.Command, args []string) error {
			if !ExportTemplate {
				if len(args) < 2 {
					return errors.New("requires at least two args")
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if Html && Json {
				panic(fmt.Errorf("error: do not use --html and --json together"))
			}

			var output *string
			if len(OutPut) > 0 {
				output = &OutPut
			}
			var template *string
			if len(Template) > 0 {
				template = &Template
			}
			if ExportTemplate {
				tmpl := ""
				if Html {
					tmpl = report.HTMLTemplate
				} else {
					tmpl = report.TextTemplate
				}
				if output != nil {
					err := os.WriteFile(*output, []byte(tmpl), 0644)
					if err != nil {
						panic(err)
					}
				} else {
					fmt.Println(tmpl)
				}
				return
			}
			assignmentConfig := config.GetAssignmentConfig(args[0], args[1], args[2:]...)
			c := gitlab.NewClient()
			if Json {
				c.ReportJSON(assignmentConfig, output)
			} else if Html {
				c.ReportHTML(assignmentConfig, template, output)
			} else {
				c.Report(assignmentConfig, template, output)
			}
		}}
	Html           bool
	Json           bool
	Template       string
	ExportTemplate bool
	OutPut         string
)
