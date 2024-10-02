package consensus

import (
	"github.com/peerdns/peerdns/pkg/encryption"
	"log"
)

func init() {
	if err := encryption.InitBLS(); err != nil {
		log.Fatalf("Failed to initialize BLS library: %v", err)
	}
}
