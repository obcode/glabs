package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func publickeys() (*ssh.PublicKeys, error) {
	privateKeyFile := fmt.Sprintf("%s/.ssh/id_rsa", os.Getenv("HOME"))
	// if pkf := startercode["privatekeyfile"]; pkf != "" {
	// 	privateKeyFile = pkf
	// }

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
