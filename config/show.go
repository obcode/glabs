package config

import (
	"fmt"
	"strings"

	"github.com/gookit/color"
)

func (cfg *AssignmentConfig) Show() {
	containerRegistry := "disabled"
	if cfg.ContainerRegistry {
		containerRegistry = "enabled"
	}

	startercode := "---"
	if cfg.Startercode != nil {
		startercode = fmt.Sprintf(`
  URL:              %s
  FromBranch:       %s
  ToBranch:         %s
  ProtectToBranch:  %t`,
			cfg.Startercode.URL,
			cfg.Startercode.FromBranch,
			cfg.Startercode.ToBranch,
			cfg.Startercode.ProtectToBranch,
		)
	}

	var per strings.Builder
	switch cfg.Per {
	case PerStudent:
		per.WriteString("Students:\n")
		for _, s := range cfg.Students {
			per.WriteString("  - ")
			per.WriteString(s)
			per.WriteString("\n")
		}
	case PerGroup:
		per.WriteString("Groups:\n")
		for _, grp := range cfg.Groups {
			per.WriteString("  - ")
			per.WriteString(grp.Name)
			per.WriteString(": ")
			for i, m := range grp.Members {
				per.WriteString(m)
				if i == len(grp.Members)-1 {
					per.WriteString("\n")
				} else {
					per.WriteString(", ")
				}
			}
		}
	}

	groupsOrStudents := per.String()

	color.Cyan.Printf(`
Course:             %s
Assignment:         %s
Per:                %s
Base-URL:           %s
Description:	    %s
AccessLevel:        %s
Container-Registry: %s
Startercode:%s
%s
`,
		cfg.Course,
		cfg.Name,
		cfg.Per,
		cfg.URL,
		cfg.Description,
		cfg.AccessLevel.show(),
		containerRegistry,
		startercode,
		groupsOrStudents,
	)

}
