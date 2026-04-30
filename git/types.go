package git

import (
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type SourceRepo struct {
	Repo *git.Repository
	Ref  plumbing.ReferenceName
	Auth ssh.AuthMethod
}
