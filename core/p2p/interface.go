package p2p

import (
	"github.com/c3systems/c3/core/chain/mainchain"
	"github.com/c3systems/c3/core/chain/statechain"

	cid "github.com/ipfs/go-cid"
)

// Interface ...
type Interface interface {
	Props() Props
	Set(v interface{}) (*cid.Cid, error)
	SetMainchainBlock(block *mainchain.Block) (*cid.Cid, error)
	SetStatechainBlock(block *statechain.Block) (*cid.Cid, error)
	SetStatechainTransaction(tx *statechain.Transaction) (*cid.Cid, error)
	SetStatechainDiff(d *statechain.Diff) (*cid.Cid, error)
	//SaveLocal(v interface{}) (*cid.Cid, error)
	//SaveLocalMainchainBlock(block *mainchain.Block) (*cid.Cid, error)
	//SaveLocalStatechainBlock(block *statechain.Block) (*cid.Cid, error)
	//SaveLocalStatechainTransaction(tx *statechain.Transaction) (*cid.Cid, error)
	//SaveLocalStatechainDiff(d *statechain.Diff) (*cid.Cid, error)
	Get(c *cid.Cid) (interface{}, error)
	GetMainchainBlock(c *cid.Cid) (*mainchain.Block, error)
	GetStatechainBlock(c *cid.Cid) (*statechain.Block, error)
	GetStatechainTransaction(c *cid.Cid) (*statechain.Transaction, error)
	GetStatechainDiff(c *cid.Cid) (*statechain.Diff, error)
}
