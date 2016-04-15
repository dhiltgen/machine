package kv

// libkv layer for machine
// holds globals and initial functions for libkv usage
// put this here to avoid import loops in other code areas

import (
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/log"
)

const MachineKvPrefix = "machine/v0"

var kvStore store.Store

func Connect(BaseDir string) (err error) {
	// TODO - figure out how to get TLS support in here...
	etcd.Register()
	kvStore, err = libkv.NewStore(store.ETCD,
		[]string{BaseDir},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)

	return err
}

func KvList(dir string) (kvList []*store.KVPair, err error) {
	log.Debugf("Looking for kv data at %s", dir)
	kvList, err = kvStore.List(dir)
	if err == store.ErrKeyNotFound {
		return kvList, nil
	}

	log.Debugf("Got data: %s", kvList)
	return kvList, err
}

func KvLoad(key string) (kvPair *store.KVPair, err error) {
	kvPair, err = kvStore.Get(key)
	return kvPair, err
}
