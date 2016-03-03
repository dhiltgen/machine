package cert

import (
	"fmt"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
)

var (
	Store CertStore
)

func BootstrapCertificates(authOptions *auth.Options) error {

	// Check the cert path, and wire up the right store for access operations
	store, err := NewCertStore(authOptions)
	if err != nil {
		return err
	}
	Store = store

	caCertPath := authOptions.CaCertPath
	caPrivateKeyPath := authOptions.CaPrivateKeyPath
	clientCertPath := authOptions.ClientCertPath
	clientKeyPath := authOptions.ClientKeyPath

	// TODO: I'm not super happy about this use of "org", the user should
	// have to specify it explicitly instead of implicitly basing it on
	// $USER.
	caOrg := mcnutils.GetUsername()
	org := caOrg + ".<bootstrap>"

	bits := 2048

	if !Store.Exists(caCertPath) {
		log.Infof("Creating CA: %s", caCertPath)

		if err := GenerateCACertificate(caCertPath, caPrivateKeyPath, caOrg, bits); err != nil {
			return fmt.Errorf("Generating CA certificate failed: %s", err)
		}
	}

	if !Store.Exists(clientCertPath) {
		log.Infof("Creating client certificate: %s", clientCertPath)

		if err := GenerateCert([]string{""}, clientCertPath, clientKeyPath, caCertPath, caPrivateKeyPath, org, bits); err != nil {
			return fmt.Errorf("Generating client certificate failed: %s", err)
		}
	}

	return nil
}
