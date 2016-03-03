package persist

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/host"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnerror"
	"github.com/docker/machine/libmachine/mcnutils"
)

const MachinePrefix = "machine/v0"

func init() {
	etcd.Register()
	//consul.Register()
	//zookeeper.Register()
	//boltdb.Register()
}

type Kvstore struct {
	storeURL url.URL
	store    store.Store
	prefix   string
}

func (s Kvstore) stripKV(path string) string {
	prefix := fmt.Sprintf("%s://%s", s.storeURL.Scheme, s.storeURL.Host)
	p := strings.TrimPrefix(path, prefix)
	// HACK - we still have places that are doing filepath join and screwing up the URLs
	prefix2 := fmt.Sprintf("%s:/%s", s.storeURL.Scheme, s.storeURL.Host)
	p = strings.TrimPrefix(p, prefix2)
	return p
}

func NewKvstore(path string, certsDir string) *Kvstore {
	log.Debugf("XXX NewKvstore(%s, %s)", path, certsDir)
	var kvStore store.Store
	kvurl, err := url.Parse(path)
	if err != nil {
		panic(fmt.Sprintf("Malformed store path: %s %s", path, err))
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
		panic(fmt.Sprintf("Unsupporetd KV store type: %s", kvurl.Scheme))
	}

	return &Kvstore{
		storeURL: *kvurl,
		store:    kvStore,
		//prefix:   kvurl.Path, // TODO - this doesn't work - needs more cleanup
	}
}

func (s Kvstore) Save(host *host.Host) error {
	data, err := json.Marshal(host)
	if err != nil {
		return err
	}

	hostPath := s.stripKV(mcnutils.Join(s.GetMachinesDir(), host.Name, "config.json"))
	log.Debugf("XXX SaveHost -> %s", hostPath)
	err = s.store.Put(hostPath, data, nil)
	return err
}

func (s Kvstore) Exists(name string) (bool, error) {
	hostPath := s.stripKV(mcnutils.Join(s.GetMachinesDir(), name, "config.json"))
	log.Debugf("XXX Exists -> %s", hostPath)
	return s.store.Exists(hostPath)
}

func (s Kvstore) loadConfig(h *host.Host, data []byte) error {
	// Remember the machine name so we don't have to pass it through each
	// struct in the migration.
	name := h.Name

	migratedHost, migrationPerformed, err := host.MigrateHost(h, data)
	if err != nil {
		return fmt.Errorf("Error getting migrated host: %s", err)
	}

	*h = *migratedHost

	h.Name = name

	// If we end up performing a migration, we should save afterwards so we don't have to do it again on subsequent invocations.
	log.Debugf("AK: migration performed: %v", migrationPerformed)
	if migrationPerformed {
		// XXX TODO do we want to save?

		//		if err := s.saveToFile(data, filepath.Join(s.GetMachinesDir(), h.Name, "config.json.bak")); err != nil {
		//			return fmt.Errorf("Error attempting to save backup after migration: %s", err)
		//		}
		//
		//		if err := s.Save(h); err != nil {
		//			return fmt.Errorf("Error saving config after migration was performed: %s", err)
		//		}
	}

	return nil
}

func (s Kvstore) Load(name string) (*host.Host, error) {
	log.Debugf("XXX Load input name is -> %s", name)
	hostPath := s.stripKV(mcnutils.Join(s.GetMachinesDir(), name, "config.json"))
	log.Debugf("XXX Load -> %s", hostPath)

	if exists, err := s.Exists(name); err != nil || exists != true {
		return nil, mcnerror.ErrHostDoesNotExist{
			Name: name,
		}
	}

	kvPair, err := s.store.Get(hostPath)
	if err != nil {
		return nil, err
	}

	host := &host.Host{
		Name: name,
	}

	if err := s.loadConfig(host, kvPair.Value); err != nil {
		return nil, err
	}

	return host, nil
}
func (s Kvstore) List() ([]string, error) {
	machineDir := s.stripKV(s.GetMachinesDir())
	log.Debugf("Looking for machines at %s", machineDir)
	kvList, err := s.store.List(machineDir)
	if err == store.ErrKeyNotFound {
		// No machines set up
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}

	hostNames := []string{}

	for _, kvPair := range kvList {
		log.Debugf("Found %s", kvPair.Key)
		hostNames = append(hostNames, path.Base(kvPair.Key))
	}

	return hostNames, nil
}

func (s Kvstore) Remove(name string) error {
	hostDir := s.stripKV(mcnutils.Join(s.GetMachinesDir(), name))
	log.Debugf("XXX Remove -> %s", hostDir)

	err := s.store.DeleteTree(hostDir)
	return err
}

func (s Kvstore) GetMachinesDir() string {
	url2 := s.storeURL
	url2.Path = mcnutils.Join(s.prefix, MachinePrefix, "machines")
	return url2.String()
}

func (s Kvstore) ListMachineFiles(host *host.Host) ([]string, error) {
	machineDir := s.stripKV(mcnutils.Join(s.GetMachinesDir(), host.Name))
	log.Debugf("Looking for machines at %s", machineDir)
	kvList, err := s.store.List(machineDir)
	if err == store.ErrKeyNotFound {
		// No machines set up
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}

	fileNames := []string{}

	for _, kvPair := range kvList {
		log.Debugf("Found %s", kvPair.Key)
		fileNames = append(fileNames, mcnutils.Join(host.Name, path.Base(kvPair.Key)))
	}

	return fileNames, nil
}
