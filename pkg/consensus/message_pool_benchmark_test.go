// message_pool_benchmark_test.go
package consensus

import (
	"fmt"
	"testing"

	"github.com/peerdns/peerdns/pkg/packets"
)

// generateBlockHash generates a unique 32-byte block hash based on the iteration index.
func generateBlockHash(i int) [32]byte {
	var hash [32]byte
	copy(hash[:], fmt.Sprintf("blockhash-%d", i))
	return hash
}

// BenchmarkMessagePool_AddAndGet benchmarks the AddMessage and GetMessage operations.
func BenchmarkMessagePool_AddAndGet(b *testing.B) {
	gLog, _ := setupTestEnvironment(b, "error")

	// Create a new MessagePool.
	mp := NewMessagePool(gLog)

	// Reset the timer to exclude setup time.
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Generate a unique block hash.
		blockHash := generateBlockHash(i)

		// Create a sample ConsensusPacket.
		msg := &packets.ConsensusPacket{
			BlockHash: blockHash[:], // Assuming BlockHash is a byte slice.
			// Populate other fields as necessary.
		}

		// Add the message to the pool.
		mp.AddMessage(msg)

		// Retrieve the message from the pool.
		retrievedMsg, err := mp.GetMessage(msg.BlockHash)
		if err != nil {
			b.Fatalf("Failed to get message: %v", err)
		}

		// Optional: Validate the retrieved message.
		if retrievedMsg != msg {
			b.Fatalf("Retrieved message does not match the original")
		}

		// Remove the message from the pool to prevent unbounded growth.
		mp.RemoveMessage(msg.BlockHash)
	}
}

// BenchmarkMessagePool_HighConcurrency benchmarks the MessagePool under high concurrency.
func BenchmarkMessagePool_HighConcurrency(b *testing.B) {
	gLog, _ := setupTestEnvironment(b, "error")

	// Create a new MessagePool.
	mp := NewMessagePool(gLog)

	// Reset the timer to exclude setup time.
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Generate a unique block hash.
			blockHash := generateBlockHash(i)
			i++

			// Create a sample ConsensusPacket.
			msg := &packets.ConsensusPacket{
				BlockHash: blockHash[:], // Assuming BlockHash is a byte slice.
				// Populate other fields as necessary.
			}

			// Add the message to the pool.
			mp.AddMessage(msg)

			// Optionally, check for the message's existence.
			if !mp.HasMessage(msg.BlockHash) {
				b.Errorf("Message should exist but does not")
			}

			// Retrieve the message from the pool.
			retrievedMsg, err := mp.GetMessage(msg.BlockHash)
			if err != nil {
				b.Errorf("Failed to get message: %v", err)
			}

			// Optional: Validate the retrieved message.
			if retrievedMsg != msg {
				b.Errorf("Retrieved message does not match the original")
			}

			// Remove the message from the pool.
			mp.RemoveMessage(msg.BlockHash)
		}
	})
}

// BenchmarkMessagePool_ListMessages benchmarks the ListMessages operation.
func BenchmarkMessagePool_ListMessages(b *testing.B) {
	gLog, _ := setupTestEnvironment(b, "error")

	// Create a new MessagePool.
	mp := NewMessagePool(gLog)

	// Pre-populate the pool with a fixed number of messages.
	prepopulateCount := 10000
	for i := 0; i < prepopulateCount; i++ {
		blockHash := generateBlockHash(i)
		msg := &packets.ConsensusPacket{
			BlockHash: blockHash[:], // Assuming BlockHash is a byte slice.
			// Populate other fields as necessary.
		}
		mp.AddMessage(msg)
	}

	// Reset the timer to exclude setup time.
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// List all messages in the pool.
		msgs := mp.ListMessages()

		// Optional: Validate the number of messages.
		if len(msgs) != prepopulateCount {
			b.Fatalf("Expected %d messages, got %d", prepopulateCount, len(msgs))
		}
	}
}

// BenchmarkMessagePool_AddRemove benchmarks adding and removing messages.
func BenchmarkMessagePool_AddRemove(b *testing.B) {
	gLog, _ := setupTestEnvironment(b, "error")

	// Create a new MessagePool.
	mp := NewMessagePool(gLog)

	// Reset the timer to exclude setup time.
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Generate a unique block hash.
		blockHash := generateBlockHash(i)

		// Create a sample ConsensusPacket.
		msg := &packets.ConsensusPacket{
			BlockHash: blockHash[:], // Assuming BlockHash is a byte slice.
			// Populate other fields as necessary.
		}

		// Add the message to the pool.
		mp.AddMessage(msg)

		// Remove the message from the pool.
		mp.RemoveMessage(msg.BlockHash)
	}
}
