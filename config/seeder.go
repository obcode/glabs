package config

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func seeder(assignmentKey string) (*Seeder, error) {
	seederMap := viper.GetStringMapString(assignmentKey + ".seeder")

	if len(seederMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no seeder provided")
		return nil, nil
	}

	cmd, ok := seederMap["cmd"]
	if !ok {
		return nil, fmt.Errorf("%s: seeder provided without cmd", assignmentKey)
	}

	toBranch := "main"
	if tB := viper.GetString(assignmentKey + ".seeder.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	entity, err := parseSignKey(assignmentKey, viper.GetString(assignmentKey+".seeder.signKey"))
	if err != nil {
		return nil, err
	}

	return &Seeder{
		Command:         cmd,
		Args:            viper.GetStringSlice(assignmentKey + ".seeder.args"),
		Name:            viper.GetString(assignmentKey + ".seeder.name"),
		SignKey:         entity,
		EMail:           viper.GetString(assignmentKey + ".seeder.email"),
		ToBranch:        toBranch,
		ProtectToBranch: viper.GetBool(assignmentKey + ".seeder.protectToBranch"),
	}, nil
}
