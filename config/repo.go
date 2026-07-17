package config

func defaultBranch(rules []BranchRule, fallback string) string {
	for _, rule := range rules {
		if rule.Default && rule.Name != "" {
			return rule.Name
		}
	}
	if fallback != "" {
		return fallback
	}
	if len(rules) > 0 {
		return rules[0].Name
	}
	return "main"
}

func (cfg *AssignmentConfig) SetBranch(branch string) {
	cfg.Clone.Branch = branch
}

func (cfg *AssignmentConfig) SetProtectToBranch(branch string) {
	if branch == "" && len(cfg.Branches) > 0 {
		branch = cfg.Branches[0].Name
	}
	if branch == "" {
		branch = "main"
	}

	for i := range cfg.Branches {
		if cfg.Branches[i].Name == branch {
			cfg.Branches[i].Protect = true
			return
		}
	}

	cfg.Branches = append(cfg.Branches, BranchRule{Name: branch, Protect: true})
}

func (cfg *AssignmentConfig) SetLocalpath(localpath string) {
	cfg.Clone.LocalPath = localpath
}

func (cfg *AssignmentConfig) SetForce() {
	cfg.Clone.Force = true
}
