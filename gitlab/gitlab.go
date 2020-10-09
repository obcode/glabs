package gitlab

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/xanzy/go-gitlab"
)

type Client struct {
	*gitlab.Client
}

func NewClient() *Client {
	log.Debug().Str("gitlab.host", viper.GetString("gitlab.host")).Msg("connecting to gitlab server")

	client, err := gitlab.NewClient(viper.GetString("gitlab.token"),
		gitlab.WithBaseURL(viper.GetString("gitlab.host")))

	if err != nil {
		panic("cannot create a gitlab client")
	}

	return &Client{client}
}
