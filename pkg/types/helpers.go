package types

import (
	"encoding/binary"
	"time"
)

// Helper function to get the current Unix timestamp.
func unixTimestamp() int64 {
	return time.Now().Unix()
}

// Helper function to convert a uint64 to a byte slice.
func uint64ToBytes(num uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, num)
	return b
}
