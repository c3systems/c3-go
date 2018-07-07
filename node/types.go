package node

import (
	"context"

	floodsub "github.com/libp2p/go-floodsub"
	host "github.com/libp2p/go-libp2p-host"

	"github.com/c3systems/c3/core/chain"
	nodestore "github.com/c3systems/c3/node/store"
)

// Props ...
type Props struct {
	CTX        context.Context
	CH         chan interface{}
	Host       host.Host
	Store      nodestore.Interface // store is used to temporarily store blocks and txs for mining and verification
	Blockchain chain.Interface
	Pubsub     *floodsub.PubSub
	//Wallet     wallet.Interface
}

// Service ...
type Service struct {
	props Props
}