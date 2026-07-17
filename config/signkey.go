package config

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/logrusorgru/aurora"
	"golang.org/x/term"
)

// parseSignKey turns an inline armored PGP private key into an entity, asking
// for the passphrase on the terminal if it is encrypted.
//
// The terminal prompt is why this is worth naming: it blocks on stdin from
// inside what is otherwise a pure config read. That is survivable in a CLI and
// fatal in a server, which is the one thing standing between config loading and
// being usable from a request handler. Both callers reach it only for
// assignments that actually configure seeder.signKey — no real course file does
// — so the block is latent, not live.
//
// The right fix is to load and decrypt the key in the execution path rather than
// the parse path, and to prefer a `signKeyFile` over a secret pasted into a
// config file. Deferred: the seeder is deprecated and unused.
func parseSignKey(assignmentKey, armored string) (*openpgp.Entity, error) {
	if armored == "" {
		return nil, nil
	}

	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
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

	return entities[0], nil
}
