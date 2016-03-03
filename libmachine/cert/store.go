package cert

import (
	"net/url"
	"os"

	"github.com/docker/machine/libmachine/auth"
)

type CertStore interface {
	// TODO - flesh this out once we know what we need

	// This is probably too low level
	Write(filename string, data []byte, flag int, perm os.FileMode) error

	// This is probably too low level
	Read(filename string) ([]byte, error)

	Exists(filename string) bool
}

func NewCertStore(authOptions *auth.Options) (CertStore, error) {
	// Determine which type of store to generate
	storeURL, err := url.Parse(authOptions.CertDir)
	if err == nil {
		// The scheme will be blank on unix paths, might be a drive letter (single char)
		// or a multi-character scheme that libkv will hopefully handle
		if len(storeURL.Scheme) > 1 {
			return NewCertKvstore(authOptions)
		}
	}
	return NewCertFilestore(authOptions)
}
