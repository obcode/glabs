package gitlab

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type Client struct {
	*gitlab.Client
}

func NewClient() *Client {
	log.Debug().Str("gitlab.host", viper.GetString("gitlab.host")).Msg("connecting to gitlab server")

	client, err := gitlab.NewClient(viper.GetString("gitlab.token"),
		gitlab.WithBaseURL(viper.GetString("gitlab.host")))

	if err != nil {
		panicFunc("cannot create a gitlab client")
	}

	return &Client{client}
}
