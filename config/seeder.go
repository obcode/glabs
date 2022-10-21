package config

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/term"
)

func seeder(assignmentKey string) *Seeder {
	seederMap := viper.GetStringMapString(assignmentKey + ".seeder")

	if len(seederMap) == 0 {
		log.Debug().Str("assignmemtKey", assignmentKey).Msg("no seeder provided")
		return nil
	}

	cmd, ok := seederMap["cmd"]
	if !ok {
		log.Fatal().Str("assignmemtKey", assignmentKey).Msg("seeder provided without cmd")
		return nil
	}

	toBranch := "main"
	if tB := viper.GetString(assignmentKey + ".seeder.toBranch"); len(tB) > 0 {
		toBranch = tB
	}

	privKeyString := viper.GetString(assignmentKey + ".seeder.signKey")
	var entity *openpgp.Entity
	entity = nil
	if privKeyString != "" {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privKeyString))
		if err != nil {
			log.Fatal()
		}
		if entities[0].PrivateKey.Encrypted {
			fmt.Println(aurora.Blue("Passphrase for signing key is required. Please enter it now:"))
			passphrase, _ := term.ReadPassword(int(syscall.Stdin))
			err = entities[0].PrivateKey.Decrypt(passphrase)
			if err != nil {
				log.Fatal()
			}
		}
		entity = entities[0]

	}

	return &Seeder{
		Command:         cmd,
		Args:            viper.GetStringSlice(assignmentKey + ".seeder.args"),
		Name:            viper.GetString(assignmentKey + ".seeder.name"),
		SignKey:         entity,
		EMail:           viper.GetString(assignmentKey + ".seeder.email"),
		ToBranch:        toBranch,
		ProtectToBranch: viper.GetBool(assignmentKey + ".seeder.protectToBranch"),
	}
}
