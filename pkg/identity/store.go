package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"sync"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"gopkg.in/yaml.v3"
)

// Store manages the persistence of DIDs using YAML files.
type Store struct {
	cfg    config.Identity
	keys   map[peer.ID]*DID
	logger logger.Logger
	mu     sync.RWMutex
}

// NewStore initializes a new Store with the identity configuration.
func NewStore(cfg config.Identity, logger logger.Logger) (*Store, error) {
	store := &Store{
		cfg:    cfg,
		logger: logger,
		keys:   make(map[peer.ID]*DID),
	}
	// Load any preconfigured keys from the configuration
	if err := store.Load(); err != nil {
		return nil, errors.Wrap(err, "failure to load preconfigured identity keys")
	}
	return store, nil
}

// Load reads all DIDs from individual YAML files and reconstructs them in the Store's keys map.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// List all YAML files in the base path directory
	files, err := filepath.Glob(filepath.Join(s.cfg.BasePath, "*.yaml"))
	if err != nil {
		return errors.Wrap(err, "failed to list DID files")
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", file)
		}

		// Unmarshal the file content into config.Key struct
		var key config.Key
		if err := yaml.Unmarshal(data, &key); err != nil {
			return errors.Wrapf(err, "failed to unmarshal DID from file %s", file)
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

		// Reconstruct the BLS signing private key if present
		var signingPrivKey *encryption.BLSPrivateKey
		if key.SigningPrivateKey != "" {
			signingPrivKeyBytes, err := hex.DecodeString(key.SigningPrivateKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing private key from file %s", file)
			}
			signingPrivKey, err = encryption.DeserializePrivateKey(signingPrivKeyBytes)
			if err != nil {
				return errors.Wrapf(err, "failed to deserialize signing private key from file %s", file)
			}
		}

		// Reconstruct the BLS signing public key if present
		var signingPubKey *encryption.BLSPublicKey
		if key.SigningPublicKey != "" {
			signingPubKeyBytes, err := hex.DecodeString(key.SigningPublicKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing public key from file %s", file)
			}
			signingPubKey, err = encryption.DeserializePublicKey(signingPubKeyBytes)
			if err != nil {
				return errors.Wrapf(err, "failed to deserialize signing public key from file %s", file)
			}
		}

		// Construct the DID object from the loaded data
		did := NewDID(peerID, privKey, pubKey, signingPrivKey, signingPubKey, key.Name, key.Comment)

		// Store the DID in the keys map
		s.keys[peerID] = did
	}

	return nil
}

// Create generates a new DID and stores it as a YAML file.
func (s *Store) Create(name, comment string, presist bool) (*DID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if a DID already exists with this name in memory
	for _, did := range s.keys {
		if did.Name == name {
			return did, nil // Return the existing DID
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

	blsPrivateKey, blsPublicKey, blsErr := encryption.GenerateBLSKeys()
	if blsErr != nil {
		return nil, errors.Wrap(blsErr, "failed to derive BLS keys")
	}

	// Create a new DID with the derived peer ID and keys
	did := NewDID(peerID, peerPrivKey, peerPubKey, blsPrivateKey, blsPublicKey, name, comment)

	// Save the DID to a YAML file
	if presist {
		if err := s.Save(did); err != nil {
			return nil, errors.Wrap(err, "failed to store DID")
		}
	}

	// Store the DID in memory
	s.keys[peerID] = did

	return did, nil
}

// Save saves a DID to a YAML file in the configured base path, including signing keys if present.
func (s *Store) Save(did *DID) error {
	// Ensure the base directory exists
	if err := os.MkdirAll(s.cfg.BasePath, 0755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	// Marshal the private key into a byte slice
	privKeyBytes, err := crypto.MarshalPrivateKey(did.PeerPrivateKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal private key")
	}

	// Marshal the public key into a byte slice
	pubKeyBytes, err := crypto.MarshalPublicKey(did.PeerPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal public key")
	}

	// Marshal the BLS signing private key if it is present
	var signingPrivKeyHex string
	if did.SigningPrivateKey != nil {
		signingPrivKeyBytes, err := did.SigningPrivateKey.Serialize()
		if err != nil {
			return errors.Wrap(err, "failed to serialize signing private key")
		}
		signingPrivKeyHex = hex.EncodeToString(signingPrivKeyBytes)
	}

	// Marshal the BLS signing public key if it is present
	var signingPubKeyHex string
	if did.SigningPublicKey != nil {
		signingPubKeyBytes, err := did.SigningPublicKey.Serialize()
		if err != nil {
			return errors.Wrap(err, "failed to serialize signing public key")
		}
		signingPubKeyHex = hex.EncodeToString(signingPubKeyBytes)
	}

	// Create a config.Key object with all key data in hexadecimal format
	toWrite := config.Key{
		Name:              did.Name,
		PeerID:            did.PeerID,
		PeerPrivateKey:    hex.EncodeToString(privKeyBytes),
		PeerPublicKey:     hex.EncodeToString(pubKeyBytes),
		SigningPrivateKey: signingPrivKeyHex,
		SigningPublicKey:  signingPubKeyHex,
		Comment:           did.Comment,
	}

	// Save the DID metadata to a YAML file
	data, err := yaml.Marshal(toWrite)
	if err != nil {
		return errors.Wrap(err, "failed to marshal DID to YAML")
	}

	// Write the YAML data to a file named with the peer ID
	filePath := filepath.Join(s.cfg.BasePath, did.ID+".yaml")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write DID file")
	}

	// Log successful save
	s.logger.Info(
		"Successfully saved did",
		zap.String("id", did.ID),
		zap.String("name", did.Name),
		zap.String("path", filePath),
	)

	return nil
}

// Get returns a DID by its peer.ID if it exists in memory.
func (s *Store) Get(peerID peer.ID) (*DID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	did, exists := s.keys[peerID]
	if !exists {
		return nil, fmt.Errorf("DID not found for peer ID: %s", peerID.String())
	}

	return did, nil
}

// List returns a list of all DIDs stored in memory.
func (s *Store) List() ([]*DID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var dids []*DID
	for _, did := range s.keys {
		dids = append(dids, did)
	}
	return dids, nil
}

// Delete removes a DID from the storage by deleting its corresponding YAML file and removing it from memory.
func (s *Store) Delete(peerID peer.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from memory
	delete(s.keys, peerID)

	// Remove the file from the disk
	filePath := filepath.Join(s.cfg.BasePath, peerID.String()+".yaml")
	if err := os.Remove(filePath); err != nil {
		return errors.Wrap(err, "failed to delete DID file")
	}

	return nil
}
