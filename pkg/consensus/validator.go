package consensus

import (
	"crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/signatures"
)

// Validator represents an individual validator participating in the consensus.
type Validator struct {
	account *accounts.Account
}

func (v *Validator) Account() *accounts.Account {
	return v.account
}

func (v *Validator) PeerID() peer.ID {
	return v.account.PeerID
}

func (v *Validator) PublicKey() crypto.PublicKey {
	return v.account.PeerPublicKey
}

func (v *Validator) PrivateKey() crypto.PrivateKey {
	return v.account.PeerPrivateKey
}

func (v *Validator) Signer(signerType signatures.SignerType) signatures.Signer {
	return v.account.Signers[signerType]
}
