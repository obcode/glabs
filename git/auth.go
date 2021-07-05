package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/spf13/viper"
)

func GetAuth() (ssh.AuthMethod, error) {
	privateKeyFile := viper.GetString("sshprivatekey")

	if privateKeyFile == "" {
		return nil, nil
	}

	_, err := os.Stat(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read ssh key from file: %w", err)
	}

	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		return nil, fmt.Errorf("cannot generate publickeys from file %s:  %w", privateKeyFile, err)
	}

	return publicKeys, nil
}
