package ipam

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/innfi/probable-eureka/pkg/config"
)

const (
	defaultDataDir  = "/var/lib/cni/networks"
	lockFileName    = ".lock"
	allocationsFile = "allocations.json"
)

type fileStore struct {
	dir string
}

func newFileStore(cfg *config.IPAMConfig) *fileStore {
	dir := cfg.DataDir
	if dir == "" {
		dir = defaultDataDir
	}
	return &fileStore{dir: dir}
}

func (fs *fileStore) Lock() (func(), error) {
	if err := os.MkdirAll(fs.dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	lockPath := filepath.Join(fs.dir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to acquire file lock: %w", err)
	}

	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
		f.Close()
	}, nil
}

func (fs *fileStore) Load() (*AllocationStore, error) {
	allocPath := filepath.Join(fs.dir, allocationsFile)

	data, err := os.ReadFile(allocPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AllocationStore{Allocations: []Allocation{}}, nil
		}
		return nil, fmt.Errorf("failed to read allocations file: %w", err)
	}

	var store AllocationStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse allocations file: %w", err)
	}

	return &store, nil
}

func (fs *fileStore) Save(store *AllocationStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal allocations: %w", err)
	}

	allocPath := filepath.Join(fs.dir, allocationsFile)
	if err := os.WriteFile(allocPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write allocations file: %w", err)
	}

	return nil
}
