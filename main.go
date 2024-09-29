// cmd/shponu/main.go
package main

import (
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"log"
)

func main() {

	// Example logger configuration
	logConfig := config.Logger{
		Enabled:     true,
		Environment: "development", // or "production"
		Level:       "debug",       // "debug", "info", "warn", "error"
	}

	if err := logger.InitializeGlobalLogger(logConfig); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Ensure that logs are flushed before exiting
	/*	defer func() {
		if err := logger.SyncGlobalLogger(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()*/

	/*// Initialize Logger
	logger := log.New(os.Stdout, "SHPoNU: ", log.LstdFlags)

	// Initialize Storage with MDBX
	store, err := storage.NewStorage("./data", "shponu-db")
	if err != nil {
		logger.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize WaitGroup for managing goroutines
	var wg sync.WaitGroup

	// Create a context that is canceled on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Generate 10 Validator Identities
	numValidators := 10
	validators := make([]*Validator, numValidators)

	for i := 0; i < numValidators; i++ {
		// Initialize Identity Manager
		identityMgr := identity.NewIdentityManager(store)
		did := identityMgr.CreateNewDID(1000) // Initial stake of 1000 tokens
		logger.Printf("Validator %d initialized with DID: %s", i+1, did.ID)

		// Initialize Networking for the Validator
		// Assign unique ports starting from 9000
		port := 9000 + i
		p2pNetwork, err := networking.NewP2PNetwork(ctx, port, "shponu-topic", logger)
		if err != nil {
			logger.Fatalf("Failed to initialize P2P network for Validator %d: %v", i+1, err)
		}

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

		// Initialize Consensus Module
		consensusModule := consensus.NewConsensusModule(did, p2pNetwork, utilityCalc, shardMgr, privacyMgr, store, logger)
		consensusModule.Start()
		logger.Printf("Validator %d consensus module started.", i+1)

		// Store Validator Information
		validators[i] = &Validator{
			ID:           i + 1,
			DID:          did,
			P2PNetwork:   p2pNetwork,
			ConsensusMod: consensusModule,
		}
	}

	// Connect Validators in a Mesh Network
	for i := 0; i < numValidators; i++ {
		for j := i + 1; j < numValidators; j++ {
			peerAddr := validators[j].P2PNetwork.Host.Addrs()[0].String() + "/p2p/" + validators[j].P2PNetwork.Host.ID().Pretty()
			err := validators[i].P2PNetwork.ConnectPeer(peerAddr)
			if err != nil {
				logger.Printf("Validator %d failed to connect to Validator %d: %v", validators[i].ID, validators[j].ID, err)
			}
		}
	}

	// Start Message Broadcasting and Verification Loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, validator := range validators {
					// Create a message
					messageContent := []byte(fmt.Sprintf("Hello from Validator %d at %s", validator.ID, time.Now().Format(time.RFC3339)))
					// Sign the message
					signedMessage, err := consensusModule.SignMessage(messageContent)
					if err != nil {
						logger.Printf("Validator %d failed to sign message: %v", validator.ID, err)
						continue
					}
					// Broadcast the message
					err = validator.ConsensusMod.BroadcastMessage(signedMessage)
					if err != nil {
						logger.Printf("Validator %d failed to broadcast message: %v", validator.ID, err)
						continue
					}
					logger.Printf("Validator %d broadcasted message: %s", validator.ID, string(messageContent))
				}
			}
		}
	}()

	// Wait for Interrupt Signal
	<-sigs
	logger.Println("Interrupt signal received. Shutting down...")

	// Cancel Context to Stop Goroutines
	cancel()

	// Wait for All Goroutines to Finish
	wg.Wait()

	// Shutdown All Validators
	for _, validator := range validators {
		validator.ConsensusMod.Stop()
		validator.P2PNetwork.Shutdown()
	}

	logger.Println("SHPoNU node shut down gracefully.")*/
}
