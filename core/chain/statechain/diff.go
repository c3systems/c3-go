package statechain

import (
	"encoding/json"

	"github.com/c3systems/c3/common/hashing"
	"github.com/c3systems/c3/common/hexutil"
)

// NewDiff ...
func NewDiff(props *DiffProps) *Diff {
	if props == nil {
		return &Diff{}
	}

	return &Diff{
		props: *props,
	}
}

// Props ...
func (d Diff) Props() DiffProps {
	return d.props
}

// Serialize ...
func (d Diff) Serialize() ([]byte, error) {
	return json.Marshal(d.props)
}

// Deserialize ...
func (d *Diff) Deserialize(bytes []byte) error {
	if d == nil {
		return ErrNilDiff
	}

	var tmpProps DiffProps
	if err := json.Unmarshal(bytes, &tmpProps); err != nil {
		return err
	}

	d.props = tmpProps
	return nil
}

// SerializeString ...
func (d Diff) SerializeString() (string, error) {
	bytes, err := d.Serialize()
	if err != nil {
		return "", err
	}

	return hexutil.EncodeString(string(bytes)), nil
}

// DeserializeString ...
func (d *Diff) DeserializeString(hexStr string) error {
	if d == nil {
		return ErrNilDiff
	}

	str, err := hexutil.DecodeString(hexStr)
	if err != nil {
		return err
	}

	return d.Deserialize([]byte(str))
}

// CalcHash ...
func (d Diff) CalcHash() (string, error) {
	tmpDiff := Diff{
		props: DiffProps{
			Data: d.props.Data,
		},
	}

	bytes, err := tmpDiff.Serialize()
	if err != nil {
		return "", err
	}

	return hashing.HashToHexString(bytes), nil
}

// SetHash ...
func (d *Diff) SetHash() error {
	if d == nil {
		return ErrNilDiff
	}

	hash, err := d.CalcHash()
	if err != nil {
		return err
	}

	d.props.DiffHash = &hash

	return nil
}