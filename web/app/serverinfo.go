package app

import (
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/spf13/viper"
)

// ServerInfo returns the build metadata main stashed in viper (ldflags at
// release, VCS info otherwise), so the GUI can show which build is running.
func (a *App) ServerInfo() *model.ServerInfo {
	return &model.ServerInfo{
		Version: viper.GetString("Version"),
		Commit:  viper.GetString("Commit"),
		Date:    viper.GetString("Date"),
	}
}
