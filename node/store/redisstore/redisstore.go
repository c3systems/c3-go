package redisstore

import (
	"errors"

	"github.com/c3systems/c3/core/chain/statechain"
	redis "github.com/gomodule/redigo/redis"
)

const (
	transactionsMembersName = "transactions"
)

// Props ...
type Props struct {
	Pool *redis.Pool
}

// Service ...
type Service struct {
	props Props
}

// New ...
func New(props *Props) (*Service, error) {
	// 1. check props
	if props == nil {
		return nil, errors.New("props cannot be nil")
	}
	if props.Pool == nil {
		return nil, errors.New("pool is required")
	}

	// 2. ping db
	c := props.Pool.Get()
	defer c.Close()
	_, err := c.Do("PING")

	// 3. return service
	return &Service{
		props: *props,
	}, err
}

// Props ...
func (s Service) Props() Props {
	return s.props
}

// HasTx ...
func (s Service) HasTx(hash string) (bool, error) {
	c := s.props.Pool.Get()
	defer c.Close()

	return redis.Bool(c.Do("EXISTS", buildKey(hash)))
}

// GetTx ...
func (s Service) GetTx(hash string) (*statechain.Transaction, error) {
	c := s.props.Pool.Get()
	defer c.Close()

	bytesStr, err := redis.Strings(c.Do("GET", hash))
	if err != nil {
		return nil, err
	}
	if len(bytesStr) == 0 {
		return nil, nil
	}

	var tx statechain.Transaction
	err = tx.DeserializeString(bytesStr[0])
	return &tx, err
}

// GetTxs ...
func (s Service) GetTxs(hashes []string) ([]*statechain.Transaction, error) {
	c := s.props.Pool.Get()
	defer c.Close()

	keys := buildKeys(hashes)
	// get many keys in a single MGET, ask redigo for []string result
	bytesStrs, err := redis.Strings(c.Do("MGET", keys))
	if err != nil {
		return nil, err
	}

	var txs []*statechain.Transaction
	for _, bytesStr := range bytesStrs {
		var tx statechain.Transaction
		if err := tx.DeserializeString(bytesStr); err != nil {
			return nil, err
		}

		txs = append(txs, &tx)
	}

	return txs, nil
}

// RemoveTx ...
func (s Service) RemoveTx(hash string) error {
	c := s.props.Pool.Get()
	defer c.Close()

	key := buildKey(hash)
	_, err := c.Do("DEL", key)
	if err != nil {
		return err
	}

	_, err = c.Do("SREM", key)
	return err
}

// RemoveTxs ...
func (s Service) RemoveTxs(hashes []string) error {
	c := s.props.Pool.Get()
	defer c.Close()

	keys := buildKeys(hashes)
	k := make([]interface{}, len(keys))
	for i, v := range k {
		k[i] = v
	}
	_, err := c.Do("DEL", k...)
	if err != nil {
		return err
	}

	_, err = c.Do("SREM", k...)

	return err
}

// AddTx ...
func (s Service) AddTx(tx *statechain.Transaction) error {
	if tx == nil {
		return errors.New("cannot add a nil transaction")
	}

	hash, err := tx.Hash()
	if err != nil {
		return err
	}

	bytesStr, err := tx.SerializeString()
	if err != nil {
		return err
	}

	c := s.props.Pool.Get()
	defer c.Close()
	_, err = c.Do("SET", buildKey(hash), bytesStr)
	if err != nil {
		return err
	}

	_, err = c.Do("SADD", transactionsMembersName, hash)

	return err
}

// GatherTransactions ...
func (s Service) GatherTransactions() ([]*statechain.Transaction, error) {
	c := s.props.Pool.Get()
	defer c.Close()

	keys, err := redis.Strings(c.Do("SMEMBERS", transactionsMembersName))
	if err != nil {
		return nil, err
	}

	bytesStrs, err := redis.Strings(c.Do("MGET", keys))
	if err != nil {
		return nil, err
	}

	var txs []*statechain.Transaction
	for _, bytesStr := range bytesStrs {
		var tx statechain.Transaction
		if err := tx.DeserializeString(bytesStr); err != nil {
			return nil, err
		}

		txs = append(txs, &tx)
	}

	return txs, nil
}