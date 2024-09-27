// pkg/topology/errors.go
package topology

import (
	"errors"
)

// Custom errors for the topology package.
var (
	ErrPeerNotFound       = errors.New("peer not found in topology")
	ErrNodeAlreadyExists  = errors.New("node already exists in topology")
	ErrInvalidMessageType = errors.New("invalid message type")
)
