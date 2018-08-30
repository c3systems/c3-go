package cmd

import (
	"errors"
	"time"

	"github.com/c3systems/c3-go/common/c3crypto"
	"github.com/c3systems/c3-go/core/chain/statechain"
	"github.com/c3systems/c3-go/node"
	nodetypes "github.com/c3systems/c3-go/node/types"
	log "github.com/sirupsen/logrus"
)

func broadcastTx(txType, image, payloadStr, peer, privPEM string) (string, error) {
	nodeURI := "/ip4/0.0.0.0/tcp/9911"
	dataDir := "~/.c3-2"
	n := new(node.Service)
	ready := make(chan bool)
	// TODO: send directly to peer node
	go func() {
		go func() {
			err := node.Start(n, &nodetypes.Config{
				URI:     nodeURI,
				Peer:    peer,
				DataDir: dataDir,
				Keys: nodetypes.Keys{
					PEMFile:  privPEM,
					Password: "",
				},
			})

			if err != nil {
				log.Fatal(err)
			}
		}()

		time.Sleep(10 * time.Second)
		ready <- true
	}()

	<-ready

	priv, err := c3crypto.ReadPrivateKeyFromPem(privPEM, nil)
	if err != nil {
		return "", err
	}

	pub, err := c3crypto.GetPublicKey(priv)
	if err != nil {
		return "", err
	}

	encodedPub, err := c3crypto.EncodeAddress(pub)
	if err != nil {
		return "", err
	}

	payload := []byte(payloadStr)

	tx := statechain.NewTransaction(&statechain.TransactionProps{
		ImageHash: image,
		Method:    txType,
		Payload:   payload,
		From:      encodedPub,
	})

	err = tx.SetHash()
	if err != nil {
		return "", err
	}

	err = tx.SetSig(priv)
	if err != nil {
		return "", err
	}

	resp, err := n.BroadcastTransaction(tx)
	if err != nil {
		return "", err
	}

	if resp.TxHash == nil {
		return "", errors.New("expected hash")
	}

	time.Sleep(3 * time.Second)
	return *resp.TxHash, err
}