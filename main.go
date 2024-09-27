// cmd/shponu/main.go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"shponu/pkg/consensus"
	"shponu/pkg/identity"
	"shponu/pkg/networking"
	"shponu/pkg/privacy"
	"shponu/pkg/sharding"
	"shponu/pkg/storage"
	"shponu/pkg/utility"
)

func main() {
	// Initialize Logger
	logger := log.New(os.Stdout, "SHPoNU: ", log.LstdFlags)

	// Initialize Storage with MDBX
	store := storage.NewStorage("./data", "shponu-db")
	defer store.Close()

	// Initialize Identity Manager
	identityMgr := identity.NewIdentityManager(store)
	node := identityMgr.CreateNewDID(1000) // Initial stake of 1000 tokens
	logger.Printf("Node initialized with DID: %s", node.ID)

	// Initialize Networking
	network := networking.NewP2PNetwork(9000, "shponu-topic", logger)
	defer network.Shutdown()

	// Initialize Utility Calculator with initial weights
	initialWeights := utility.Metrics{
		Bandwidth:      1.0,
		Computational:  1.0,
		Storage:        1.0,
		Uptime:         1.0,
		Responsiveness: 1.0,
		Reliability:    1.0,
	}
	utilityCalc := utility.NewUtilityCalculator(initialWeights)

	// Initialize Shard Manager
	shardMgr := sharding.NewShardManager()

	// Initialize Privacy Manager
	privacyMgr := privacy.NewPrivacyManager()

	// Initialize Consensus
	consensusModule := consensus.NewConsensus(node, network, utilityCalc, shardMgr, privacyMgr, store)
	consensusModule.Start()

	// Handle OS signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	logger.Println("Shutting down SHPoNU node...")
}
