package config

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/term"
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

	privKeyString := viper.GetString(assignmentKey + ".seeder.signKey")
	var entity *openpgp.Entity
	if privKeyString != "" {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privKeyString))
		if err != nil {
			return nil, fmt.Errorf("%s: cannot read seeder.signKey as an armored PGP key ring: %w", assignmentKey, err)
		}
		if len(entities) == 0 {
			return nil, fmt.Errorf("%s: seeder.signKey contains no PGP key", assignmentKey)
		}
		if entities[0].PrivateKey == nil {
			return nil, fmt.Errorf("%s: seeder.signKey contains no private key", assignmentKey)
		}
		if entities[0].PrivateKey.Encrypted {
			fmt.Println(aurora.Blue("Passphrase for signing key is required. Please enter it now:"))
			passphrase, _ := term.ReadPassword(int(syscall.Stdin))
			if err := entities[0].PrivateKey.Decrypt(passphrase); err != nil {
				return nil, fmt.Errorf("%s: cannot decrypt seeder.signKey with the given passphrase: %w", assignmentKey, err)
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
	}, nil
}
