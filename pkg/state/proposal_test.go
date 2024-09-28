package state

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProposalEncodeDecode(t *testing.T) {
	proposal := &Proposal{
		ID:        "test-proposal",
		BlockHash: []byte{0x1, 0x2, 0x3},
		Content:   []byte("Test Content"),
		Timestamp: time.Now().Unix(),
	}

	encoded, err := proposal.Encode()
	require.NoError(t, err, "Failed to encode proposal")

	decoded, err := DecodeProposal(encoded)
	require.NoError(t, err, "Failed to decode proposal")
	require.Equal(t, proposal, decoded, "Decoded proposal should match original")
}
