package ipam

import "github.com/innfi/probable-eureka/pkg/config"

// Store handles persistence of IP allocations.
type Store interface {
	// Lock acquires exclusive access; the returned func releases it.
	Lock() (func(), error)
	// Load reads the current allocation state.
	Load() (*AllocationStore, error)
	// Save writes the allocation state.
	Save(*AllocationStore) error
}

// NewStore returns a Store for the configured backend ("file" by default, "etcd" if specified).
func NewStore(cfg *config.IPAMConfig) (Store, error) {
	switch cfg.Backend {
	case "etcd":
		return newEtcdStore(cfg)
	default:
		return newFileStore(cfg), nil
	}
}
