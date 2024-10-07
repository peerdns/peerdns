package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
	"os"
	"path/filepath"
	"sync"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Store manages the persistence of Accounts using YAML files.
type Store struct {
	cfg       config.Identity
	keys      map[peer.ID]*Account
	addresses map[types.Address]*Account
	logger    logger.Logger
	mu        sync.RWMutex
}

// NewStore initializes a new Store with the identity configuration.
func NewStore(cfg config.Identity, logger logger.Logger) (*Store, error) {
	store := &Store{
		cfg:       cfg,
		logger:    logger,
		keys:      make(map[peer.ID]*Account),
		addresses: make(map[types.Address]*Account),
	}
	// Load any preconfigured keys from the configuration
	if err := store.Load(); err != nil {
		return nil, errors.Wrap(err, "failure to load preconfigured identity keys")
	}
	return store, nil
}

// Load reads all Accounts from individual YAML files and reconstructs them in the Store's keys map.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// List all YAML files in the base path directory
	files, err := filepath.Glob(filepath.Join(s.cfg.BasePath, "*.yaml"))
	if err != nil {
		return errors.Wrap(err, "failed to list identity files")
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", file)
		}

		// Unmarshal the file content into config.Key struct
		var key config.Key
		if err := yaml.Unmarshal(data, &key); err != nil {
			return errors.Wrapf(err, "failed to unmarshal identity from file %s", file)
		}

		// Decode the peer ID from the key
		peerID, err := peer.Decode(key.PeerID.String())
		if err != nil {
			return errors.Wrapf(err, "failed to decode peer ID from file %s", file)
		}

		// Decode and reconstruct the private key
		privKeyBytes, err := hex.DecodeString(key.PeerPrivateKey)
		if err != nil {
			return errors.Wrapf(err, "failed to decode private key from file %s", file)
		}
		privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
		if err != nil {
			return errors.Wrapf(err, "failed to unmarshal private key from file %s", file)
		}

		// Decode and reconstruct the public key
		pubKeyBytes, err := hex.DecodeString(key.PeerPublicKey)
		if err != nil {
			return errors.Wrapf(err, "failed to decode public key from file %s", file)
		}
		pubKey, err := crypto.UnmarshalPublicKey(pubKeyBytes)
		if err != nil {
			return errors.Wrapf(err, "failed to unmarshal public key from file %s", file)
		}

		// Reconstruct the signers
		signers := make(map[signatures.SignerType]signatures.Signer)
		var address types.Address

		for signerTypeStr, signerKey := range key.Signers {
			signerType := signatures.SignerType(signerTypeStr)
			privateKeyBytes, err := hex.DecodeString(signerKey.SigningPrivateKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing private key for signer type %s from file %s", signerType, file)
			}
			publicKeyBytes, err := hex.DecodeString(signerKey.SigningPublicKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing public key for signer type %s from file %s", signerType, file)
			}

			var signer signatures.Signer
			switch signerType {
			case signatures.BlsSignerType:
				signer, err = signatures.NewBLSSignerWithKeys(privateKeyBytes, publicKeyBytes)
			case signatures.Ed25519SignerType:
				signer, err = signatures.NewEd25519SignerWithKeys(privateKeyBytes, publicKeyBytes)
				if err != nil {
					return errors.Wrapf(err, "failed to initialize Ed25519 signer from file %s", file)
				}
				// Compute the address from the Ed25519 public key
				address, err = computeAddressFromPublicKey(publicKeyBytes)
				if err != nil {
					return errors.Wrapf(err, "failed to compute address from public key in file %s", file)
				}
			case signatures.DleqSignerType:
				signer, err = signatures.NewZKProofSignerWithKeys(privateKeyBytes, publicKeyBytes)
			default:
				return fmt.Errorf("unsupported signer type: %s", signerType)
			}
			if err != nil {
				return errors.Wrapf(err, "failed to initialize %s signer from file %s", signerType, file)
			}
			signers[signerType] = signer
		}

		// Construct the Account object from the loaded data
		account := NewAccount(peerID, privKey, pubKey, signers, key.Name, key.Comment)
		account.Address = address

		// Store the Account in the keys and addresses maps
		s.keys[peerID] = account
		s.addresses[address] = account
	}

	return nil
}

// Create generates a new Account and stores it as a YAML file.
func (s *Store) Create(name, comment string, persist bool) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if an Account already exists with this name in memory
	for _, account := range s.keys {
		if account.Name == name {
			return account, nil // Return the existing Account
		}
	}

	// Generate RSA keypair for peer.ID using a proper random source
	peerPrivKey, peerPubKey, err := crypto.GenerateRSAKeyPair(2048, rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate RSA keys for peer ID")
	}

	// Derive the peer.ID from the RSA public key
	peerID, err := peer.IDFromPublicKey(peerPubKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive peer ID")
	}

	// Initialize signers map
	signers := make(map[signatures.SignerType]signatures.Signer)

	// Generate BLS signer
	blsSigner, err := signatures.NewBLSSigner()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create BLS signer")
	}
	signers[signatures.BlsSignerType] = blsSigner

	// Generate Ed25519 signer
	ed25519Signer, err := signatures.NewEd25519Signer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Ed25519 signer")
	}
	signers[signatures.Ed25519SignerType] = ed25519Signer

	// Generate DLEQ signer
	dleqSigner, err := signatures.NewZKProofSigner()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DLEQ signer")
	}
	signers[signatures.DleqSignerType] = dleqSigner

	// Create a new Account with the derived peer ID and keys
	account := NewAccount(peerID, peerPrivKey, peerPubKey, signers, name, comment)

	// Compute the address from the Ed25519 public key
	publicKeyBytes, err := ed25519Signer.Pair().SerializePublic()
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize Ed25519 public key")
	}
	address, err := computeAddressFromPublicKey(publicKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute address from public key")
	}
	account.Address = address

	// Save the Account to a YAML file
	if persist {
		if err := s.Save(account); err != nil {
			return nil, errors.Wrap(err, "failed to store Account")
		}
	}

	// Store the Account in memory
	s.keys[peerID] = account
	s.addresses[address] = account
	return account, nil
}

// Save saves an Account to a YAML file in the configured base path, including signing keys if present.
func (s *Store) Save(account *Account) error {
	// Ensure the base directory exists
	if err := os.MkdirAll(s.cfg.BasePath, 0755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	// Marshal the private key into a byte slice
	privKeyBytes, err := crypto.MarshalPrivateKey(account.PeerPrivateKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal private key")
	}

	// Marshal the public key into a byte slice
	pubKeyBytes, err := crypto.MarshalPublicKey(account.PeerPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal public key")
	}

	// Serialize the signers
	signers := make(map[signatures.SignerType]config.SignerKey)
	var address types.Address

	for signerType, signer := range account.Signers {
		var privateKeyBytes []byte
		var publicKeyBytes []byte

		switch s := signer.(type) {
		case *signatures.BLSSigner:
			privateKeyBytes, err = s.Pair().SerializePrivate()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize private key for signer type %s", signerType)
			}
			publicKeyBytes, err = s.Pair().SerializePublic()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize public key for signer type %s", signerType)
			}
		case *signatures.Ed25519Signer:
			privateKeyBytes, err = s.Pair().SerializePrivate()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize private key for signer type %s", signerType)
			}
			publicKeyBytes, err = s.Pair().SerializePublic()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize public key for signer type %s", signerType)
			}
			// Compute the Address field from the Ed25519 public key
			address, err = computeAddressFromPublicKey(publicKeyBytes)
			if err != nil {
				return errors.Wrap(err, "failed to compute address from public key")
			}
		case *signatures.ZKProofSigner:
			privateKeyBytes, err = s.Pair().SerializePrivate()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize private key for signer type %s", signerType)
			}
			publicKeyBytes, err = s.Pair().SerializePublic()
			if err != nil {
				return errors.Wrapf(err, "failed to serialize public key for signer type %s", signerType)
			}
		default:
			return fmt.Errorf("unsupported signer type: %s", signerType)
		}

		signerKey := config.SignerKey{
			SigningPrivateKey: hex.EncodeToString(privateKeyBytes),
			SigningPublicKey:  hex.EncodeToString(publicKeyBytes),
		}
		signers[signerType] = signerKey
	}

	// Update the account's address
	account.Address = address

	// Create a config.Key object with all key data in hexadecimal format
	toWrite := config.Key{
		Name:           account.Name,
		PeerID:         account.PeerID,
		PeerPrivateKey: hex.EncodeToString(privKeyBytes),
		PeerPublicKey:  hex.EncodeToString(pubKeyBytes),
		Signers:        signers,
		Address:        address, // Set the Address field
		Comment:        account.Comment,
	}

	// Save the Account metadata to a YAML file
	data, err := yaml.Marshal(toWrite)
	if err != nil {
		return errors.Wrap(err, "failed to marshal Account to YAML")
	}

	// Write the YAML data to a file named with the peer ID
	filePath := filepath.Join(s.cfg.BasePath, account.ID+".yaml")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write Account file")
	}

	// Log successful save
	s.logger.Info(
		"Successfully saved account",
		zap.String("id", account.ID),
		zap.String("name", account.Name),
		zap.String("path", filePath),
	)

	return nil
}

// GetByPeerID returns an Account by its peer.ID if it exists in memory.
func (s *Store) GetByPeerID(peerID peer.ID) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.keys[peerID]
	if !exists {
		return nil, fmt.Errorf("account not found for peer ID: %s", peerID.String())
	}

	return account, nil
}

// GetByAddress returns an Account by its Address if it exists in memory.
func (s *Store) GetByAddress(addr types.Address) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.addresses[addr]
	if !exists {
		return nil, fmt.Errorf("account not found for address: %s", addr.Hex())
	}

	return account, nil
}

// List returns a list of all Accounts stored in memory.
func (s *Store) List() ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var accounts []*Account
	for _, account := range s.keys {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// Delete removes an Account from the storage by deleting its corresponding YAML file and removing it from memory.
func (s *Store) Delete(peerID peer.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the Account exists in memory
	_, exists := s.keys[peerID]
	if !exists {
		return fmt.Errorf("Account not found for peer ID: %s", peerID.String())
	}

	// Remove from memory
	delete(s.keys, peerID)

	// Remove the file from the disk
	filePath := filepath.Join(s.cfg.BasePath, peerID.String()+".yaml")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to delete Account file")
	}

	return nil
}
