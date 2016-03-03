package cert

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/log"
)

const MachinePrefix = "machine/v0"

func init() {
	etcd.Register()
	//consul.Register()
	//zookeeper.Register()
	//boltdb.Register()
}

type CertKvstore struct {
	storeURL    url.URL
	authOptions *auth.Options
	store       store.Store
	prefix      string
}

func NewCertKvstore(authOptions *auth.Options) (*CertKvstore, error) {
	log.Debugf("XXX NewCertKvstore(%#v)", authOptions)
	var kvStore store.Store
	kvurl, err := url.Parse(authOptions.CertDir)
	if err != nil {
		return nil, fmt.Errorf("Malformed store path: %s %s", authOptions.CertDir, err)
	}
	switch kvurl.Scheme {
	case "etcd":
		// TODO - figure out how to get TLS support in here...
		kvStore, err = libkv.NewStore(
			store.ETCD,
			[]string{kvurl.Host},
			&store.Config{
				ConnectionTimeout: 10 * time.Second,
			},
		)
		// TODO other KV store types
	default:
		return nil, fmt.Errorf("Unsupporetd KV store type: %s", kvurl.Scheme)
	}

	// XXX This feels super messy - there's got to be a better way.
	//prefix := fmt.Sprintf("%s:/%s", kvurl.Scheme, kvurl.Host)
	//authOptions.CertDir = strings.TrimPrefix(authOptions.CertDir, prefix)
	//authOptions.CaCertPath = strings.TrimPrefix(authOptions.CaCertPath, prefix)
	//authOptions.CaPrivateKeyPath = strings.TrimPrefix(authOptions.CaPrivateKeyPath, prefix)
	//authOptions.CaCertRemotePath = strings.TrimPrefix(authOptions.CaCertRemotePath, prefix)
	//authOptions.ServerCertPath = strings.TrimPrefix(authOptions.ServerCertPath, prefix)
	//authOptions.ServerKeyPath = strings.TrimPrefix(authOptions.ServerKeyPath, prefix)
	//authOptions.ClientKeyPath = strings.TrimPrefix(authOptions.ClientKeyPath, prefix)
	//authOptions.ServerCertRemotePath = strings.TrimPrefix(authOptions.ServerCertRemotePath, prefix)
	//authOptions.ServerKeyRemotePath = strings.TrimPrefix(authOptions.ServerKeyRemotePath, prefix)
	//authOptions.ClientCertPath = strings.TrimPrefix(authOptions.ClientCertPath, prefix)
	//authOptions.StorePath = strings.TrimPrefix(authOptions.StorePath, prefix)
	log.Debugf("XXX CertDir: %s", authOptions.CertDir)
	log.Debugf("XXX CaCertPath: %s", authOptions.CaCertPath)

	return &CertKvstore{
		storeURL: *kvurl,
		store:    kvStore,
		//prefix:      kvurl.Path, // TODO - needs more work to get the sequencing right
		authOptions: authOptions,
	}, nil
}

func (s CertKvstore) Write(filename string, data []byte, flag int, perm os.FileMode) error {
	// TODO - this pattern needs cleanup
	prefix := fmt.Sprintf("%s://%s", s.storeURL.Scheme, s.storeURL.Host)
	filename = strings.TrimPrefix(filename, prefix)

	key := filepath.Join("/", MachinePrefix, s.prefix, filename)
	log.Debugf("XXX CertKvstore.Write -> %s", key)
	err := s.store.Put(key, data, nil)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (s CertKvstore) Read(filename string) ([]byte, error) {
	prefix := fmt.Sprintf("%s://%s", s.storeURL.Scheme, s.storeURL.Host)
	filename = strings.TrimPrefix(filename, prefix)
	key := filepath.Join("/", MachinePrefix, s.prefix, filename)
	log.Debugf("XXX CertKvstore.Read -> %s", key)
	kvpair, err := s.store.Get(key)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return kvpair.Value, nil
}
func (s CertKvstore) Exists(filename string) bool {
	prefix := fmt.Sprintf("%s://%s", s.storeURL.Scheme, s.storeURL.Host)
	filename = strings.TrimPrefix(filename, prefix)
	key := filepath.Join("/", MachinePrefix, s.prefix, filename)
	log.Debugf("XXX CertKvstore.Exists -> %s", key)
	exists, err := s.store.Exists(key)
	if err != nil {
		// TODO log a better message on other errors
		log.Errorf("KV lookup failure on %s: %s", filename, err)
		return false
	}
	if err != nil {
		log.Error(err)
	}
	log.Debugf("XXX Exists: %v", exists)

	return exists
}
