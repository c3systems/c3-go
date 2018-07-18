package miner

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c3systems/c3/common/hexutil"
	"github.com/c3systems/c3/core/chain/mainchain"
	"github.com/c3systems/c3/core/chain/merkle"
	"github.com/c3systems/c3/core/chain/statechain"
	"github.com/c3systems/c3/core/diffing"
	"github.com/c3systems/c3/core/p2p"
	"github.com/c3systems/c3/core/sandbox"
	methodTypes "github.com/c3systems/c3/core/types/methods"
	"github.com/c3systems/c3/logger"
	"github.com/c3systems/merkletree"
)

// New returns a new service
func New(props *Props) (*Service, error) {
	if props == nil {
		return nil, errors.New("props are required")
	}

	statechainBlocksMap := make(map[string]*statechain.Block)
	transactionsMap := make(map[string]*statechain.Transaction)
	diffsMap := make(map[string]*statechain.Diff)
	merkleTreesMap := make(map[string]*merkle.Tree)

	s := &Service{
		props: *props,
		minedBlock: &MinedBlock{
			NextBlock:           nil,
			PreviousBlock:       props.PreviousBlock,
			mut:                 sync.Mutex{},
			StatechainBlocksMap: statechainBlocksMap,
			TransactionsMap:     transactionsMap,
			DiffsMap:            diffsMap,
			MerkleTreesMap:      merkleTreesMap,
		},
	}

	nextBlock, err := s.bootstrapNextBlock()
	if err != nil {
		return nil, err
	}
	s.minedBlock.NextBlock = nextBlock

	return s, nil
}

// Props returns the props
func (s Service) Props() Props {
	return s.props
}

// SpawnMiner ...
func (s Service) SpawnMiner() error {
	// TODO: reward ourselves with some coin
	go func() {
		var (
			err error
		)

		switch s.props.Async {
		case true:
			err = s.buildMainchainBlockAsync()
			if err != nil {
				s.props.Channel <- err
				return
			}

		default:
			err = s.buildMainchainBlock()
			if err != nil {
				s.props.Channel <- err
				return
			}
		}

		if s.minedBlock == nil || s.minedBlock.NextBlock == nil {
			s.props.Channel <- errors.New("built a nil block")
			return
		}

		s.props.Channel <- s.minedBlock
	}()

	return nil
}

func (s Service) buildMainchainBlockAsync() error {
	var (
		wg  sync.WaitGroup
		err error
	)

	// 1. gather tx's
	// TODO: only choose high value tx's to mine
	txsMap := BuildTxsMap(s.props.PendingTransactions)

	log.Printf("[miner] build mainchain block async; tx count: %v", len(txsMap))

	// 2. apply txs
	for imageHash, transactions := range txsMap {
		wg.Add(1)
		go func(iHash string, txs []*statechain.Transaction) {
			defer wg.Done()
			if s.props.Context.Err() != nil {
				err = s.props.Context.Err()

				return
			}

			if err1 := s.buildNextStates(iHash, txs); err1 != nil {
				// err = err1 note: don't do this, we'll just skip this image hash
				log.Printf("[miner] err mining state block for hash %s transactions %v: %v", iHash, txs, err1)
				return
			}
		}(imageHash, transactions)
	}
	wg.Wait()

	if err != nil {
		return err
	}

	// 3. mine main block
	return s.mineBlock()
}

func (s Service) buildMainchainBlock() error {
	// 1. gather tx's
	// TODO: only choose high value tx's to mine
	txsMap := BuildTxsMap(s.props.PendingTransactions)

	log.Printf("[miner] build mainchain block; tx count: %v", len(txsMap))

	// 2. apply txs
	for imageHash, transactions := range txsMap {
		if s.props.Context.Err() != nil {
			return s.props.Context.Err()
		}

		if err := s.buildNextStates(imageHash, transactions); err != nil {
			log.Printf("[miner] err mining state block for hash %s transactions %v: %v", imageHash, transactions, err)
			continue
		}
	}

	// 3. mine main block
	return s.mineBlock()
}

func (s Service) mineBlock() error {
	// TODO: timeout?
	if err := s.generateMerkle(); err != nil {
		log.Printf("[miner] error mining block; %s", err)
		return err
	}

	for {
		if s.props.Context.Err() != nil {
			return s.props.Context.Err()
		}

		hash, nonce, err := s.generateHashAndNonce()
		if err != nil {
			log.Printf("[miner] error generating hash and nonce; %s", err)
			return err
		}

		check, err := CheckHashAgainstDifficulty(hash, s.props.Difficulty)
		if err != nil {
			log.Printf("[miner] error checking hash against difficulty; %s", err)
			return err
		}

		if check {
			nextProps := s.minedBlock.NextBlock.Props()
			nextProps.Nonce = nonce
			nextBlock := mainchain.New(&nextProps)
			s.minedBlock.NextBlock = nextBlock

			log.Println("[miner] difficulty checks out")
			return s.minedBlock.NextBlock.SetHash()
		}

		// note: else the for loop continues and we try the next hash
	}
}

func (s Service) generateMerkle() error {
	log.Println("[miner] generating merkle")
	var (
		hashes []string
		list   []merkletree.Content
	)

	log.Printf("[miner] state chain blocks length; %v", len(s.minedBlock.StatechainBlocksMap))
	for _, statechainBlock := range s.minedBlock.StatechainBlocksMap {
		if s.props.Context.Err() != nil {
			return s.props.Context.Err()
		}

		if statechainBlock == nil {
			log.Println("[miner] state chain block is nil")
			return errors.New("nil block")
		}
		if statechainBlock.Props().BlockHash == nil {
			log.Println("[miner] state chain block hash is nil")
			return errors.New("nil block hash")
		}

		hashes = append(hashes, *statechainBlock.Props().BlockHash)
		list = append(list, statechainBlock)
	}

	tree, err := merkle.BuildFromObjects(list, merkle.StatechainBlocksKindStr)
	if err != nil {
		log.Printf("[miner] error building merkle tree from objects\n%v", list)
		return err
	}
	if tree == nil {
		log.Println("[miner] tree is nil")
		return errors.New("nil tree")
	}
	if tree.Props().MerkleTreeRootHash == nil {
		log.Println("[miner] merkle root hash is nil")
		return errors.New("nil merkle root hash")
	}

	s.minedBlock.mut.Lock()
	s.minedBlock.MerkleTreesMap[*tree.Props().MerkleTreeRootHash] = tree
	s.minedBlock.mut.Unlock()

	nextProps := s.minedBlock.NextBlock.Props()
	nextProps.StateBlocksMerkleHash = *tree.Props().MerkleTreeRootHash
	nextBlock := mainchain.New(&nextProps)
	s.minedBlock.NextBlock = nextBlock

	return nil
}

func (s Service) generateHashAndNonce() (string, string, error) {
	nonce, err := s.generateNonce()
	if err != nil {
		return "", "", err
	}

	tmpProps := s.minedBlock.NextBlock.Props()
	tmpProps.Nonce = nonce
	tmpBlock := mainchain.New(&tmpProps)

	hash, err := tmpBlock.CalculateHash()
	return hash, nonce, err
}

func (s Service) generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hexutil.EncodeString(string(bytes)), nil
}

func (s Service) bootstrapNextBlock() (*mainchain.Block, error) {
	nextProps := new(mainchain.Props)

	nextProps.BlockNumber = hexutil.EncodeUint64(0)
	nextProps.BlockTime = hexutil.EncodeUint64(uint64(time.Now().Unix()))
	nextProps.Difficulty = hexutil.EncodeUint64(s.props.Difficulty)
	nextProps.MinerAddress = s.props.EncodedMinerAddress

	// previous block will be nil if first block
	if s.props.PreviousBlock != nil {
		prevProps := s.props.PreviousBlock.Props()
		if prevProps.BlockHash == nil {
			return nil, errors.New("previous block's block hash is nil")
		}

		// note: checked for nil block hash, above
		nextProps.PrevBlockHash = *prevProps.BlockHash
		prevBlockHeight, err := hexutil.DecodeUint64(prevProps.BlockNumber)
		if err != nil {
			return nil, err
		}

		nextProps.BlockNumber = hexutil.EncodeUint64(prevBlockHeight + 1)
	}

	return mainchain.New(nextProps), nil
}

// note: the transactions array are all for the image hash, passed
func (s Service) buildNextStates(imageHash string, transactions []*statechain.Transaction) error {
	var (
		diffs          []*statechain.Diff
		prevStateBlock *statechain.Block
	)

	log.Printf("[miner] build next state state; image hash: %s, tx count: %v", imageHash, len(transactions))

	// note: pops tx from transactions if genesisTransaction
	isGenesisTx, tx, transactions, err := isGenesisTransaction(s.props.P2P, s.minedBlock.PreviousBlock, imageHash, transactions)
	if err != nil {
		log.Printf("[miner] error determining if tx is genesis for image hash %s; error: %s", imageHash, err)
		return err
	}
	if isGenesisTx {
		log.Printf("[miner] is genesis tx for image hash %s", imageHash)
		genesisBlock, diff, err := buildGenesisStateBlock(imageHash, tx)
		if err != nil {
			return err
		}

		// write to the mined block
		s.minedBlock.mut.Lock()
		s.minedBlock.DiffsMap[*diff.Props().DiffHash] = diff
		s.minedBlock.TransactionsMap[*tx.Props().TxHash] = tx
		s.minedBlock.StatechainBlocksMap[*genesisBlock.Props().BlockHash] = genesisBlock
		s.minedBlock.mut.Unlock()
		log.Printf("[miner] mined state block for image hash %s", imageHash)

		prevStateBlock = genesisBlock
		diffs = append(diffs, diff)
	} else {
		prevStateBlock, err = s.props.P2P.FetchMostRecentStateBlock(imageHash, s.props.PreviousBlock)
		if err != nil {
			log.Printf("[miner] error fetching most recent state block for image hash %s %s", imageHash, err)
			return err
		}

		// gather the diffs
		diffs, err = s.gatherDiffs(prevStateBlock)
		if err != nil {
			log.Printf("[miner] error getting cid by hash for image hash %s\n%v", imageHash, err)
			return err
		}
		log.Printf("[miner] total diffs %v", len(diffs))
	}

	// apply the diffs to get the current state
	genesisState := []byte("")
	state, err := generateStateFromDiffs(s.props.Context, imageHash, genesisState, diffs)
	if err != nil {
		log.Printf("[miner] error getting state from diffs for image hash %s\n%v", imageHash, err)
		return err
	}

	newStatechainBlocks, newDiffs, err := s.buildStateblocksAndDiffsFromStateAndTransactions(prevStateBlock, imageHash, state, transactions)
	if err != nil {
		log.Printf("[miner] error building state blocks from state and txs for image hash %s\n%v", imageHash, err)
		return err
	}

	// write to the mined block
	s.minedBlock.mut.Lock()
	defer s.minedBlock.mut.Unlock()
	// note: they should all have same length
	for i := 0; i < len(newDiffs); i++ {
		s.minedBlock.DiffsMap[*newDiffs[i].Props().DiffHash] = newDiffs[i]
		s.minedBlock.TransactionsMap[*transactions[i].Props().TxHash] = transactions[i]
		s.minedBlock.StatechainBlocksMap[*newStatechainBlocks[i].Props().BlockHash] = newStatechainBlocks[i]
	}

	return nil
}

func (s *Service) gatherDiffs(block *statechain.Block) ([]*statechain.Diff, error) {
	var diffs []*statechain.Diff

	if block == nil {
		log.Printf("[miner] can't gather diffs because block is nil; returning empty list")
		return diffs, nil
	}

	diffCID, err := p2p.GetCIDByHash(block.Props().StatePrevDiffHash)
	if err != nil {
		return nil, err
	}

	diff, err := s.props.P2P.GetStatechainDiff(diffCID)
	if err != nil {
		return nil, err
	}
	// note: prepend
	diffs = append([]*statechain.Diff{diff}, diffs...)

	head := block
	for head.Props().BlockNumber != mainchain.GenesisBlock.Props().BlockNumber {
		if s.props.Context.Err() != nil {
			return nil, s.props.Context.Err()
		}

		prevStateCID, err := p2p.GetCIDByHash(head.Props().PrevBlockHash)
		if err != nil {
			return nil, err
		}

		prevStateBlock, err := s.props.P2P.GetStatechainBlock(prevStateCID)
		if err != nil {
			return nil, err
		}
		head = prevStateBlock

		diffCID, err := p2p.GetCIDByHash(prevStateBlock.Props().StatePrevDiffHash)
		if err != nil {
			return nil, err
		}
		diff, err := s.props.P2P.GetStatechainDiff(diffCID)
		if err != nil {
			return nil, err
		}
		// note: prepend
		diffs = append([]*statechain.Diff{diff}, diffs...)
	}

	return diffs, nil
}

func (s *Service) buildStateblocksAndDiffsFromStateAndTransactions(prevStateBlock *statechain.Block, imageHash string, state []byte, transactions []*statechain.Transaction) ([]*statechain.Block, []*statechain.Diff, error) {
	var (
		newDiffs            []*statechain.Diff
		newStatechainBlocks []*statechain.Block
		fileNames           []string
	)
	defer cleanupFiles(fileNames)

	ts := time.Now().Unix()

	tmpStateFile, err := makeTempFile(fmt.Sprintf("%s/%v/state.txt", imageHash, ts))
	if err != nil {
		return nil, nil, err
	}
	fileNames = append(fileNames, tmpStateFile.Name())
	if _, err := tmpStateFile.Write(state); err != nil {
		return nil, nil, err
	}
	if err := tmpStateFile.Close(); err != nil {
		return nil, nil, err
	}

	patchFile, err := makeTempFile(fmt.Sprintf("%s/%v/state.patch", imageHash, ts))
	if err != nil {
		return nil, nil, err
	}
	fileNames = append(fileNames, patchFile.Name())
	if err := tmpStateFile.Close(); err != nil {
		return nil, nil, err
	}

	headStateFileName := tmpStateFile.Name()
	runningBlockNumber, err := hexutil.DecodeUint64(prevStateBlock.Props().BlockNumber)
	if err != nil {
		return nil, nil, err
	}
	runningBlockHash := *prevStateBlock.Props().BlockHash // note: already checked nil pointer, above

	// apply state to container and start running transactions
	for i, tx := range transactions {
		if s.props.Context.Err() != nil {
			return nil, nil, s.props.Context.Err()
		}

		if tx == nil {
			log.Printf("[miner] tx is nil for image hash %s", imageHash)
			return nil, nil, errors.New("nil tx")
		}
		if tx.Props().TxHash == nil {
			log.Printf("[miner] tx hash is nil for %v", tx.Props())
			return nil, nil, errors.New("nil tx hash")
		}

		var nextState []byte
		log.Printf("[miner] tx method %s", tx.Props().Method)

		if tx.Props().Method == methodTypes.InvokeMethod {
			payload := tx.Props().Payload

			var parsed []string
			if err := json.Unmarshal(payload, &parsed); err != nil {
				log.Printf("[miner] error unmarshalling json for image hash %s", imageHash)
				return nil, nil, err
			}

			inputsJSON, err := json.Marshal(struct {
				Method string   `json:"method"`
				Params []string `json:"params"`
			}{
				Method: parsed[0],
				Params: parsed[1:],
			})
			if err != nil {
				log.Printf("[miner] error marshalling json for image hash %s", imageHash)
				return nil, nil, err
			}

			log.Printf("[miner] invoking method %s for image hash %s", parsed[0], imageHash)

			// run container, passing the tx inputs
			sb := sandbox.NewSandbox(&sandbox.Config{})
			nextState, err = sb.Play(&sandbox.PlayConfig{
				ImageID:      tx.Props().ImageHash,
				Payload:      inputsJSON,
				InitialState: state,
			})

			if err != nil {
				log.Printf("[miner] error running container for image hash: %s; error: %s", tx.Props().ImageHash, err)
				return nil, nil, err
			}

			log.Printf("[miner] container new state: %s", string(nextState))
		}

		nextStateFile, err := makeTempFile(fmt.Sprintf("%s/%v/state.%d.txt", imageHash, ts, i))
		if err != nil {
			return nil, nil, err
		}
		fileNames = append(fileNames, nextStateFile.Name()) // clean up

		if _, err := nextStateFile.Write(nextState); err != nil {
			return nil, nil, err
		}
		if err := nextStateFile.Close(); err != nil {
			return nil, nil, err
		}

		if err := diffing.Diff(headStateFileName, nextStateFile.Name(), patchFile.Name(), false); err != nil {
			return nil, nil, err
		}
		headStateFileName = nextStateFile.Name()

		// build the diff struct
		diffData, err := ioutil.ReadFile(patchFile.Name())
		if err != nil {
			return nil, nil, err
		}

		diffStruct := statechain.NewDiff(&statechain.DiffProps{
			Data: string(diffData),
		})
		if err := diffStruct.SetHash(); err != nil {
			return nil, nil, err
		}

		nextStateHash := hexutil.EncodeBytes(nextState)
		runningBlockNumber++
		nextStateStruct := statechain.New(&statechain.BlockProps{
			BlockNumber:       hexutil.EncodeUint64(runningBlockNumber),
			BlockTime:         hexutil.EncodeUint64(uint64(ts)),
			ImageHash:         imageHash,
			TxHash:            *tx.Props().TxHash, // note: checked for nil pointer, above
			PrevBlockHash:     runningBlockHash,
			StatePrevDiffHash: *diffStruct.Props().DiffHash, // note: used setHash, above so it would've erred
			StateCurrentHash:  string(nextStateHash),
		})
		if err := nextStateStruct.SetHash(); err != nil {
			return nil, nil, err
		}
		runningBlockHash = *nextStateStruct.Props().BlockHash

		newDiffs = append(newDiffs, diffStruct)
		newStatechainBlocks = append(newStatechainBlocks, nextStateStruct)
	}

	return newStatechainBlocks, newDiffs, nil
}

func init() {
	log.AddHook(logger.ContextHook{})
}
