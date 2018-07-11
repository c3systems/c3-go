package node

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/c3systems/c3/core/chain/mainchain"
	"github.com/c3systems/c3/core/chain/mainchain/miner"
	"github.com/c3systems/c3/core/chain/statechain"
	"github.com/c3systems/c3/core/p2p"
	"github.com/c3systems/c3/core/p2p/store/fsstore"
	"github.com/c3systems/c3/node/store/safemempool"
	nodetypes "github.com/c3systems/c3/node/types"

	ipfsaddr "github.com/ipfs/go-ipfs-addr"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	floodsub "github.com/libp2p/go-floodsub"
	libp2p "github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
)

// Start ...
// note: start is called from cobra
func Start(cfg *nodetypes.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg == nil {
		// note: is this the correct way to fail an app with cobra?
		return errors.New("config is required to start the node")
	}

	newNode, err := libp2p.New(ctx, libp2p.Defaults, libp2p.ListenAddrStrings(cfg.URI))
	if err != nil {
		return fmt.Errorf("err building libp2p service\n%v", err)
	}

	pubsub, err := floodsub.NewFloodSub(ctx, newNode)
	if err != nil {
		return fmt.Errorf("err building new pubsub service\n%v", err)
	}

	for i, addr := range newNode.Addrs() {
		log.Printf("%d: %s/ipfs/%s\n", i, addr, newNode.ID().Pretty())
	}

	if cfg.Peer != "" {
		addr, err := ipfsaddr.ParseString(cfg.Peer)
		if err != nil {
			return fmt.Errorf("err parsing node uri flag: %s\n%v", cfg.URI, err)
		}

		pinfo, err := peerstore.InfoFromP2pAddr(addr.Multiaddr())
		if err != nil {
			return fmt.Errorf("err getting info from peerstore\n%v", err)
		}

		if err := newNode.Connect(ctx, *pinfo); err != nil {
			log.Printf("bootstrapping a peer failed\n%v", err)
		}
	}

	// TODO: add cli flags for different types
	memPool, err := safemempool.New(&safemempool.Props{})
	if err != nil {
		return fmt.Errorf("err initializing mempool\n%v", err)
	}
	// TODO: ping the network for the newest block. For now we always start at 0
	if err := memPool.SetHeadBlock(mainchain.GenesisBlock); err != nil {
		return fmt.Errorf("err setting head block\n%v", err)
	}

	diskStore, err := fsstore.New(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("err building disk store\n%v", err)
	}
	// wrap the datastore in a 'content addressed blocks' layer
	// TODO: implement metrics? https://github.com/ipfs/go-ds-measure
	blocks := bstore.NewBlockstore(diskStore)

	p2pSvc, err := p2p.New(&p2p.Props{
		BlockStore: blocks,
		Host:       newNode,
	})
	if err != nil {
		return fmt.Errorf("err starting ipfs p2p network\n%v", err)
	}

	n, err := New(&Props{
		Context:             ctx,
		SubscriberChannel:   make(chan interface{}),
		CancelMinersChannel: make(chan struct{}),
		Host:                newNode,
		Store:               memPool,
		Pubsub:              pubsub,
		P2P:                 p2pSvc,
	})
	if err != nil {
		return fmt.Errorf("err building the node\n%v", err)
	}

	if err := n.listenForEvents(); err != nil {
		return fmt.Errorf("err starting listener\n%v", err)
	}
	// TODO: add a cli flag to determine if the node mines
	if err := n.spawnNextBlockMiner(&mainchain.GenesisBlock); err != nil {
		return fmt.Errorf("err starting miner\n%v", err)
	}
	log.Printf("Node %s started", newNode.ID().Pretty())

	for {
		switch v := <-n.props.SubscriberChannel; v.(type) {
		case error:
			log.Println("[node] received an error on the channel", err)

		case *miner.MinedBlock:
			log.Print("[node] received mined block")
			b, _ := v.(*miner.MinedBlock)
			go n.handleReceiptOfMainchainBlock(b)

		case *statechain.Transaction:
			log.Print("[node] received statechain transaction")
			tx, _ := v.(*statechain.Transaction)
			go n.handleReceiptOfStatechainTransaction(tx)
			// TODO: move this to the miner and handle multiple tx's
			// handleTransaction(tx)

		default:
			log.Printf("[node] received an unknown message on channel of type %T\n%v", v, v)
		}
	}
}

// TODO: move this to the miner
// handleTransaction performs container actions after receiving tx
//func handleTransaction(tx *statechain.Transaction) error {
//data := tx.Props()
//if data.Method == "c3_invokeMethod" {
//payload, ok := data.Payload.([]byte)
//if !ok {
//return errors.New("could not parse payload")
//}

//var parsed []string
//if err := json.Unmarshal(payload, &parsed); err != nil {
//return err
//}

//inputsJSON, err := json.Marshal(struct {
//Method string   `json:"method"`
//Params []string `json:"params"`
//}{
//Method: parsed[0],
//Params: parsed[1:],
//})
//if err != nil {
//return err
//}

//// run container, passing the tx inputs
//sb := sandbox.NewSandbox(&sandbox.Config{})
//if err := sb.Play(&sandbox.PlayConfig{
//ImageID: data.ImageHash,
//Payload: inputsJSON,
//}); err != nil {
//return err
//}
//}

//return nil
//}
