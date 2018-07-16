package miner

import (
	"testing"

	"github.com/c3systems/c3/common/c3crypto"
	"github.com/c3systems/c3/core/chain/statechain"
)

func TestBuildGenesisStateBlock(t *testing.T) {
	imageHash := "QmQpXfKvirguQaMG7khqvLrqWcxEzh2qVApfC1Ts7QyFK7"

	privPEM := "./test_data/priv.pem"
	priv, err := c3crypto.ReadPrivateKeyFromPem(privPEM, nil)
	if err != nil {
		t.Error(err)
	}

	pub, err := c3crypto.GetPublicKey(priv)
	if err != nil {
		t.Error(err)
	}

	encodedPub, err := c3crypto.EncodeAddress(pub)
	if err != nil {
		t.Error(err)
	}

	tx := statechain.NewTransaction(&statechain.TransactionProps{
		ImageHash: imageHash,
		Method:    "c3_invokeMethod",
		Payload:   []byte(`[""setItem", "foo", "bar"]`),
		From:      encodedPub,
	})
	err = tx.SetHash()
	if err != nil {
		t.Error(err)
	}

	txs := []*statechain.Transaction{tx}
	mnr, err := New(&Props{
		IsValid:             nil,
		PreviousBlock:       nil,
		Difficulty:          uint64(5),
		Channel:             make(chan interface{}),
		Async:               true,
		EncodedMinerAddress: "",
		PendingTransactions: txs,
	})

	if err != nil {
		t.Error(err)
	}

	err = mnr.buildGenesisStateBlock(imageHash, txs[0])
	if err != nil {
		t.Error(err)
	}
}