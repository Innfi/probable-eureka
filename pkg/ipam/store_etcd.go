package ipam

import (
	"context"
	"encoding/json"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/innfi/probable-eureka/pkg/config"
)

const (
	etcdAllocKey = "probable-eureka/ipam/allocations"
	etcdLockKey  = "probable-eureka/ipam/lock"
)

type etcdStore struct {
	client *clientv3.Client
}

func newEtcdStore(cfg *config.IPAMConfig) (*etcdStore, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: cfg.EtcdEndpoints,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}
	return &etcdStore{client: cli}, nil
}

func (es *etcdStore) Lock() (func(), error) {
	sess, err := concurrency.NewSession(es.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd session: %w", err)
	}
	mu := concurrency.NewMutex(sess, etcdLockKey)
	if err := mu.Lock(context.Background()); err != nil {
		sess.Close()
		return nil, fmt.Errorf("failed to acquire etcd lock: %w", err)
	}
	return func() {
		mu.Unlock(context.Background()) //nolint:errcheck
		sess.Close()
	}, nil
}

func (es *etcdStore) Load() (*AllocationStore, error) {
	resp, err := es.client.Get(context.Background(), etcdAllocKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get allocations from etcd: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return &AllocationStore{Allocations: []Allocation{}}, nil
	}
	var store AllocationStore
	if err := json.Unmarshal(resp.Kvs[0].Value, &store); err != nil {
		return nil, fmt.Errorf("failed to unmarshal allocations from etcd: %w", err)
	}
	return &store, nil
}

func (es *etcdStore) Save(store *AllocationStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal allocations: %w", err)
	}
	_, err = es.client.Put(context.Background(), etcdAllocKey, string(data))
	if err != nil {
		return fmt.Errorf("failed to put allocations to etcd: %w", err)
	}
	return nil
}
