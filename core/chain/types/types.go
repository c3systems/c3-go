package types

import "github.com/c3systems/c3/core/p2p"

// Props ...
type Props struct {
	P2P p2p.Interface
}

// Service ...
type Service struct {
	props Props
}
