package kv

// libkv layer for machine
// holds globals and initial functions for libkv usage
// put this here to avoid import loops in other code areas

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/log"
)

const MachineKvPrefix = "machine/v0"

var kvStore store.Store

func Connect(kvHost string) (err error) {
	log.Debugf("AK: connect %d: %s", os.Getpid(), kvHost)
	// TODO - figure out how to get TLS support in here...
	if kvStore != nil {
		log.Debugf("AK: redundant connect, ignoring")
		return nil
	}
	etcd.Register()
	kvStore, err = libkv.NewStore(store.ETCD,
		[]string{kvHost},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)

	if err != nil {
		log.Warnf("AK: err in connect, this is BAD: %s", err)
	}

	return err
}

func KvList(dir string) (kvList []*store.KVPair, err error) {
	log.Debugf("AK: list %d", os.Getpid())
	if kvStore == nil {
		panic(fmt.Errorf("KVStore not initialized!!"))
	}
	kvList, err = kvStore.List(addPrefix(dir))
	if err == store.ErrKeyNotFound {
		return kvList, nil
	}

	log.Debugf("Got data: %s", kvList)
	return kvList, err
}

func KvPut(path string, data []byte) (err error) {
	log.Debugf("AK: put %d", os.Getpid())
	if kvStore == nil {
		panic(fmt.Errorf("KVStore not initialized!!"))
	}

	err = kvStore.Put(addPrefix(path), data, nil)
	return err
}

func addPrefix(key string) string {
	return path.Join(MachineKvPrefix, key)
}

func KvLoad(key string) (kvPair *store.KVPair, err error) {
	log.Debugf("AK: load %d", os.Getpid())
	if kvStore == nil {
		panic(fmt.Errorf("KVStore not initialized!!"))
	}
	kvPair, err = kvStore.Get(addPrefix(key))
	return kvPair, err
}

func KvExists(key string) (exists bool, err error) {
	log.Debugf("AK: exists %d", os.Getpid())
	if kvStore == nil {
		panic(fmt.Errorf("KVStore not initialized!!"))
	}
	log.Debugf("AK: exists? %s", key)
	exists, err = kvStore.Exists(addPrefix(key))
	if err != nil {
		log.Warnf("ERR: %s", err)
	}
	return exists, err
}
