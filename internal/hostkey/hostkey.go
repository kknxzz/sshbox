// Package hostkey persists sshbox's ssh host key across restarts.
package hostkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"

	"golang.org/x/crypto/ssh"
)

// LoadOrCreate reads the host key from path, generating and saving a new
// one if it doesn't exist yet. Without this, gliderlabs/ssh generates a
// fresh throwaway key every time the server starts, which makes every
// restart look like a different host to any client that already has the
// old key in known_hosts.
func LoadOrCreate(path string) (ssh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(data)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	block, err := ssh.MarshalPrivateKey(priv, "sshbox host key")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(priv)
}
