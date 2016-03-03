package cert

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/log"
)

type CertFilestore struct {
	Path        string
	authOptions *auth.Options
}

func NewCertFilestore(authOptions *auth.Options) (*CertFilestore, error) {
	log.Debugf("XXX NewCertFilestore(%#v)", authOptions)

	if _, err := os.Stat(authOptions.CertDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(authOptions.CertDir, 0700); err != nil {
				return nil, fmt.Errorf("Creating machine certificate dir failed: %s", err)
			}
		} else {
			return nil, err
		}
	}

	return &CertFilestore{
		Path:        authOptions.CertDir, // XXX WRONG!!!
		authOptions: authOptions,
	}, nil
}

func (s CertFilestore) Write(filename string, data []byte, flag int, perm os.FileMode) error {
	log.Debugf("XXX CertFilestore.Write(%s, <data>)", filename)

	// TODO - audit/verify this impl if we keep it
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return ioutil.WriteFile(filename, data, perm) // TODO - flag!
	}

	tmpfi, err := ioutil.TempFile(filepath.Dir(filename), ".tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfi.Name())

	if err = ioutil.WriteFile(tmpfi.Name(), data, perm); err != nil {
		return err
	}

	if err = tmpfi.Close(); err != nil {
		return err
	}

	if err = os.Remove(filename); err != nil {
		return err
	}

	if err = os.Rename(tmpfi.Name(), filename); err != nil {
		return err
	}
	return nil
}

func (s CertFilestore) Read(filename string) ([]byte, error) {
	log.Debugf("XXX CertFilestore.Read(%s, <data>)", filename)
	return ioutil.ReadFile(filename)
}

func (s CertFilestore) Exists(filename string) bool {
	log.Debugf("XXX CertFilestore.Exists(%s, <data>)", filename)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	} else if err == nil {
		return true
	} else {
		log.Errorf("Stat failure on %s: %s", filename, err)
		return false
	}
}
