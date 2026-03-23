package config

import "fmt"

func (cfg *AssignmentConfig) Urls(assignment bool) {
	if assignment {
		fmt.Println(cfg.URL)
	} else if cfg.Per == PerStudent {
		for _, stud := range cfg.Students {
			fmt.Printf("%s/%s\n", cfg.URL, cfg.RepoNameForStudent(stud))
		}
	} else { // PerGroup
		for _, group := range cfg.Groups {
			fmt.Printf("%s/%s\n", cfg.URL, cfg.RepoNameForGroup(group))
		}
	}
}
