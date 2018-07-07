package chain

import (
	"errors"

	mainchain "github.com/c3systems/c3/core/chain/mainchain"
	statechain "github.com/c3systems/c3/core/chain/statechain"
	cid "github.com/ipfs/go-cid"
)

// New ...
func New(props *Props) (*Service, error) {
	if props == nil {
		return nil, errors.New("props cannot be nil")
	}

	return &Service{
		props: *props,
	}, nil
}

// TODO: implement methods

// AddMainBlock ...
func (s Service) AddMainBlock(block *mainchain.Block) *cid.Cid {
	return nil
}

// Transactions ...
func (s Service) Transactions() []*statechain.Transaction {
	return nil
}

// MainHead ...
func (s Service) MainHead() (*mainchain.Block, error) {
	return nil, nil
}

// StateHead ...
func (s Service) StateHead(hash string) (*statechain.Block, error) {
	return nil, nil
}