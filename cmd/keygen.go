package cmd

import (
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func KeystoreCommand() *cli.Command {
	return &cli.Command{
		Name:  "keystore",
		Usage: "Keystore management utilities",
		Subcommands: []*cli.Command{
			// Generate a new identity
			{
				Name:  "generate",
				Usage: "Generate new keystore account (T-BLS) and P2P private keys",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Name of the identity",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "comment",
						Usage: "Comment or description for the identity",
					},
					&cli.BoolFlag{
						Name:     "save",
						Usage:    "Save the identity to disk",
						Required: false,
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()
					identityManager, imErr := identity.NewManager(&config.G().Identity, log)
					if imErr != nil {
						log.Error("Failed to create identity manager", zap.Error(imErr))
						return imErr
					}

					// Create new identity using provided flags
					name := cmd.String("name")
					comment := cmd.String("comment")
					save := cmd.Bool("save")

					did, dErr := identityManager.Create(name, comment, save)
					if dErr != nil {
						log.Error("Failed to create identity", zap.Error(dErr))
						return dErr
					}

					// Convert public keys to a readable format
					peerPublicKeyBytes, _ := did.PeerPublicKey.Raw()
					signingPublicKeyBytes, _ := did.SigningPublicKey.Serialize()

					// Print created identity information
					fmt.Printf("\n[Identity Created]\n")
					fmt.Printf("DID ID:             %s\n", did.ID)
					fmt.Printf("Peer ID:            %s\n", did.PeerID)
					fmt.Printf("Peer Public Key:    %s\n", hex.EncodeToString(peerPublicKeyBytes))
					fmt.Printf("Signing Public Key: %s\n", hex.EncodeToString(signingPublicKeyBytes))
					fmt.Printf("Name:               %s\n", did.Name)
					fmt.Printf("Comment:            %s\n", did.Comment)
					fmt.Printf("Saved to Disk:      %v\n\n", save)

					if save {
						fmt.Println("\nIdentity created successfully and persisted to disk.")
					}
					return nil
				},
			},

			// List all stored identities
			{
				Name:  "list",
				Usage: "List all stored identities",
				Action: func(cmd *cli.Context) error {
					log := logger.G()
					identityManager, imErr := identity.NewManager(&config.G().Identity, log)
					if imErr != nil {
						log.Error("Failed to create identity manager", zap.Error(imErr))
						return imErr
					}

					dids, err := identityManager.List()
					if err != nil {
						log.Error("Failed to list identities", zap.Error(err))
						return err
					}

					if len(dids) == 0 {
						fmt.Println("No identities found.")
						return nil
					}

					fmt.Printf("\n[Stored Identities]\n")
					for _, did := range dids {
						fmt.Printf("DID ID: %s, Name: %s, Peer ID: %s\n", did.ID, did.Name, did.PeerID)
					}

					return nil
				},
			},

			// Retrieve specific identity details by Peer ID
			{
				Name:  "get",
				Usage: "Get identity information by Peer ID",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "peer-id",
						Usage:    "Peer ID of the identity to retrieve",
						Required: true,
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()
					identityManager, imErr := identity.NewManager(&config.G().Identity, log)
					if imErr != nil {
						log.Error("Failed to create identity manager", zap.Error(imErr))
						return imErr
					}

					peerIDStr := cmd.String("peer-id")
					peerID, err := peer.Decode(peerIDStr)
					if err != nil {
						log.Error("Invalid Peer ID format", zap.Error(err))
						return err
					}

					did, err := identityManager.Get(peerID)
					if err != nil {
						log.Error("Failed to get identity", zap.Error(err))
						return err
					}

					peerPublicKeyBytes, _ := did.PeerPublicKey.Raw()
					signingPublicKeyBytes, _ := did.SigningPublicKey.Serialize()

					fmt.Printf("\n[Identity Information]\n")
					fmt.Printf("DID ID:             %s\n", did.ID)
					fmt.Printf("Peer ID:            %s\n", did.PeerID)
					fmt.Printf("Peer Public Key:    %s\n", hex.EncodeToString(peerPublicKeyBytes))
					fmt.Printf("Signing Public Key: %s\n", hex.EncodeToString(signingPublicKeyBytes))
					fmt.Printf("Name:               %s\n", did.Name)
					fmt.Printf("Comment:            %s\n", did.Comment)

					return nil
				},
			},

			// Delete an identity by Peer ID
			{
				Name:  "delete",
				Usage: "Delete identity by Peer ID",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "peer-id",
						Usage:    "Peer ID of the identity to delete",
						Required: true,
					},
					&cli.BoolFlag{
						Name:     "force",
						Usage:    "Force delete without confirmation",
						Required: false,
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()
					identityManager, imErr := identity.NewManager(&config.G().Identity, log)
					if imErr != nil {
						log.Error("Failed to create identity manager", zap.Error(imErr))
						return imErr
					}

					peerIDStr := cmd.String("peer-id")
					peerID, err := peer.Decode(peerIDStr)
					if err != nil {
						log.Error("Invalid Peer ID format", zap.Error(err))
						return err
					}

					if !cmd.Bool("force") {
						fmt.Printf("Are you sure you want to delete identity with Peer ID: %s? [y/N] ", peerIDStr)
						var response string
						fmt.Scanln(&response)
						if response != "y" && response != "Y" {
							fmt.Println("Delete operation aborted.")
							return nil
						}
					}

					if err := identityManager.Delete(peerID); err != nil {
						log.Error("Failed to delete identity", zap.Error(err))
						return err
					}

					fmt.Printf("Identity with Peer ID: %s successfully deleted.\n", peerIDStr)
					return nil
				},
			},
		},
	}
}
