package persist

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/docker/machine/libmachine/host"
	"github.com/docker/machine/libmachine/kv"
	"github.com/docker/machine/libmachine/log"
)

type Kvstore struct {
	Path string // compat
}

func NewKvstore(path string) (*Kvstore, error) {
	log.Debugf("AK: NewKvstore(%s)", path)
	err := kv.Connect(path)
	if err != nil {
		return nil, err
	}

	return &Kvstore{}, nil
}

func (s Kvstore) Save(host *host.Host) error {
	data, err := json.Marshal(host)
	if err != nil {
		return err
	}

	hostPath := path.Join(getMachineBase(host.Name), "config.json")
	return kv.KvPut(hostPath, data)
}

func (s Kvstore) Exists(name string) (bool, error) {
	machinePath := path.Join(getMachineBase(name), "config.json")
	return kv.KvExists(machinePath)
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

func getMachineBase(name string) string {
	return path.Join("machines", name)
}

func (s Kvstore) Load(name string) (loadedHost *host.Host, err error) {
	log.Debugf("XXX Load input name is -> %s", name)
	machinePath := path.Join(getMachineBase(name), "config.json")
	kvPair, err := kv.KvLoad(machinePath)
	if err != nil {
		return nil, err
	}

	loadedHost = &host.Host{
		Name: name,
	}

	if err := s.loadConfig(loadedHost, kvPair.Value); err != nil {
		return nil, err
	}

	return loadedHost, nil
}

func (s Kvstore) List() (results []string, err error) {
	kvList, err := kv.KvList("machines")
	if err != nil {
		return results, err
	}

	for _, kvPair := range kvList {
		log.Debugf("Found %s", kvPair.Key)
		results = append(results, path.Base(kvPair.Key))
	}

	return results, err
}

func (s Kvstore) Remove(name string) error {
	machinePath := getMachineBase(name)
	log.Debugf("AK: deleting %s", machinePath)
	return kv.KvDeleteTree(machinePath)
}

func (s Kvstore) GetMachinesDir() string {
	log.Warnf("XXX: NYI: GetMachinesDir (do we want this?!)")
	return ""
	//	url2 := s.storeURL
	//	url2.Path = mcnutils.Join(s.prefix, MachinePrefix, "machines")
	//	return url2.String()
}

func (s Kvstore) ListMachineFiles(host *host.Host) ([]string, error) {
	return []string{}, fmt.Errorf("NYI: ListMachineFiles")
}
