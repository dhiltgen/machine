package persist

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/host"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnerror"
)

type Filestore struct {
	Path             string
	CaCertPath       string
	CaPrivateKeyPath string
	store            store.Store
}

type Certs struct {
	CaCert     []byte
	ServerCert []byte
	ServerKey  []byte
}

func init() {
	etcd.Register()
	//consul.Register()
	//zookeeper.Register()
	//boltdb.Register()
}

func NewFilestore(path, caCertPath, caPrivateKeyPath string) *Filestore {
	log.Debugf("XXX NewFilestore(%s, %s, %s)", path, caCertPath, caPrivateKeyPath)
	kvurl, err := url.Parse(path)
	var kvStore store.Store
	if err != nil {
		switch kvurl.Scheme {
		case "etcd":
			kvStore, err = libkv.NewStore(
				store.ETCD,
				[]string{kvurl.Host},
				&store.Config{
					ConnectionTimeout: 10 * time.Second,
				},
			)
			// TODO other KV store types
		}
	}

	return &Filestore{
		Path:             path,
		CaCertPath:       caCertPath,
		CaPrivateKeyPath: caPrivateKeyPath,
		store:            kvStore,
	}
}

func (s Filestore) GetMachinesDir() string {
	return filepath.Join(s.Path, "machines")
}

func (s Filestore) saveToFile(data []byte, file string) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return ioutil.WriteFile(file, data, 0600)
	}

	tmpfi, err := ioutil.TempFile(filepath.Dir(file), "config.json.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfi.Name())

	if err = ioutil.WriteFile(tmpfi.Name(), data, 0600); err != nil {
		return err
	}

	if err = tmpfi.Close(); err != nil {
		return err
	}

	if err = os.Remove(file); err != nil {
		return err
	}

	if err = os.Rename(tmpfi.Name(), file); err != nil {
		return err
	}
	return nil
}

func (s Filestore) Save(host *host.Host) error {
	data, err := json.MarshalIndent(host, "", "    ")
	if err != nil {
		return err
	}

	hostPath := filepath.Join(s.GetMachinesDir(), host.Name)

	// Ensure that the directory we want to save to exists.
	if err := os.MkdirAll(hostPath, 0700); err != nil {
		return err
	}

	return s.saveToFile(data, filepath.Join(hostPath, "config.json"))
}

func (s Filestore) Remove(name string) error {
	hostPath := filepath.Join(s.GetMachinesDir(), name)
	return os.RemoveAll(hostPath)
}

func (s Filestore) List() ([]string, error) {
	dir, err := ioutil.ReadDir(s.GetMachinesDir())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	hostNames := []string{}

	for _, file := range dir {
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			hostNames = append(hostNames, file.Name())
		}
	}

	return hostNames, nil
}

func (s Filestore) Exists(name string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.GetMachinesDir(), name))

	if os.IsNotExist(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	}

	return false, err
}

func (s Filestore) loadConfig(h *host.Host) error {
	data, err := ioutil.ReadFile(filepath.Join(s.GetMachinesDir(), h.Name, "config.json"))
	if err != nil {
		return err
	}

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
	if migrationPerformed {
		if err := s.saveToFile(data, filepath.Join(s.GetMachinesDir(), h.Name, "config.json.bak")); err != nil {
			return fmt.Errorf("Error attempting to save backup after migration: %s", err)
		}

		if err := s.Save(h); err != nil {
			return fmt.Errorf("Error saving config after migration was performed: %s", err)
		}
	}

	return nil
}

func (s Filestore) Load(name string) (*host.Host, error) {
	hostPath := filepath.Join(s.GetMachinesDir(), name)

	if _, err := os.Stat(hostPath); os.IsNotExist(err) {
		return nil, mcnerror.ErrHostDoesNotExist{
			Name: name,
		}
	}

	host := &host.Host{
		Name: name,
	}

	if err := s.loadConfig(host); err != nil {
		return nil, err
	}

	return host, nil
}

func (s Filestore) LoadCerts(authOptions *auth.Options) (*Certs, error) {
	caCertPath := authOptions.CaCertPath
	serverCertPath := authOptions.ServerCertPath
	serverKeyPath := authOptions.ServerKeyPath

	log.Debugf("Reading CA certificate from %s", caCertPath)
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}

	log.Debugf("Reading server certificate from %s", serverCertPath)
	serverCert, err := ioutil.ReadFile(serverCertPath)
	if err != nil {
		return nil, err
	}

	log.Debugf("Reading server key from %s", serverKeyPath)
	serverKey, err := ioutil.ReadFile(serverKeyPath)
	if err != nil {
		return nil, err
	}

	certs := &Certs{
		CaCert:     caCert,
		ServerCert: serverCert,
		ServerKey:  serverKey,
	}

	return certs, nil
}
