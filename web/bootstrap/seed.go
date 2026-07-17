package bootstrap

import (
	"context"
	"strings"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// seedUsers inserts the allowlist from auth.seedusers, but only when the users
// collection is empty — so a fresh deployment has someone who can get in, while
// later config edits never resurrect a user removed from the database.
func seedUsers(ctx context.Context, database *db.DB) error {
	count, err := database.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var seed []struct {
		Email string
		Name  string
	}
	if err := viper.UnmarshalKey("auth.seedusers", &seed); err != nil {
		return err
	}
	if len(seed) == 0 {
		log.Warn().Msg("no users in the database and no auth.seedusers configured — nobody will be able to log in")
		return nil
	}

	for _, s := range seed {
		email := strings.ToLower(strings.TrimSpace(s.Email))
		if email == "" {
			continue
		}
		if err := database.SaveUser(ctx, &model.User{Email: email, Name: strings.TrimSpace(s.Name)}); err != nil {
			return err
		}
		log.Info().Str("email", email).Msg("seeded user")
	}
	return nil
}
