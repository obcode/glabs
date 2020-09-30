package gitlab

import (
	"encoding/json"
	"fmt"

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

func (c *Client) GetGroupInfo(groupname string) {
	group, resp, err := c.Groups.GetGroup(viper.GetInt(groupname + ".id"))
	if err != nil {
		log.Error().Err(err).Str("group", groupname).Msg("error while fetching group info")
	}

	groupInfo, _ := json.MarshalIndent(group, "", "  ")
	fmt.Printf("%s\n", groupInfo)
	fmt.Printf("%+v\n", resp.Response.Status)
}
