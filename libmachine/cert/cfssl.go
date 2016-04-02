package cert

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
)

var (
	Profile = "node" // What profile in CFSSL to request - TODO probably want to make this tunable...
)

type CFSSLCertGenerator struct {
	authOptions     *auth.Options
	caURL           *url.URL
	clientTlsConfig *tls.Config
	client          *http.Client
}

func NewCFSSLCertGenerator(authOptions *auth.Options) Generator {
	return &CFSSLCertGenerator{
		authOptions: authOptions,
	}
}

func (cg *CFSSLCertGenerator) Init() error {
	caURL, err := url.Parse(cg.authOptions.RemoteCa)
	if err != nil {
		return fmt.Errorf("Malformed Remote CA: %s", caURL)
	}
	cg.caURL = caURL
	tlsConfig := &tls.Config{}
	if _, err := os.Stat(cg.authOptions.CaCertPath); os.IsNotExist(err) {
		log.Debug("No CA cert detected, using system wide trusted CAs")
	} else {
		caCert, err := ioutil.ReadFile(cg.authOptions.CaCertPath)
		if err != nil {
			return fmt.Errorf("Failed to load %s: %s", cg.authOptions.CaCertPath, err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}
	if _, err := os.Stat(cg.authOptions.ClientCertPath); os.IsNotExist(err) {
		log.Debug("No client certs detected, skipping mutual TLS")
	} else {
		cert, err := tls.LoadX509KeyPair(cg.authOptions.ClientCertPath, cg.authOptions.ClientKeyPath)
		if err != nil {
			return fmt.Errorf("Failed to load existing %s: %s", cg.authOptions.ClientCertPath, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	cg.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	return nil
}

func (cg *CFSSLCertGenerator) GenerateCACertificate(certFile, keyFile, org string, bits int) error {
	return fmt.Errorf("You can not generate a CA for a remote CA")
}

func (cg *CFSSLCertGenerator) GenerateServerCert(hosts []string, authOptions *auth.Options, org string, bits int) error {
	if cg.client == nil {
		if err := cg.Init(); err != nil {
			return err
		}
	}
	log.Debug("XXX Remote CA - Generating Server Cert")
	if len(hosts) == 0 {
		return fmt.Errorf("You must specify at least one hostname")
	}
	tmpl := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   hosts[0],
			Organization: []string{org},
		},
		EmailAddresses: []string{},
		IPAddresses:    []net.IP{},
		DNSNames:       []string{},
	}
	for i := range hosts {
		if ip := net.ParseIP(hosts[i]); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, hosts[i])
		}
	}

	cert, key, err := cg.doCSR(tmpl, bits)
	if err != nil {
		return err
	}
	log.Debug("XXX Remote CA - writing out server cert and key")
	err = ioutil.WriteFile(authOptions.ServerCertPath, []byte(cert), 0644)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(authOptions.ServerKeyPath, []byte(key), 0600)
	if err != nil {
		return err
	}
	return nil
}

func (cg *CFSSLCertGenerator) GenerateClientCert(hosts []string, authOptions *auth.Options, org string, bits int) error {
	if cg.client == nil {
		if err := cg.Init(); err != nil {
			return err
		}
	}
	log.Debug("XXX Remote CA - Generating Client Cert")
	tmpl := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   mcnutils.GetUsername(),
			Country:      []string{""}, // TODO - try to remove these...
			Province:     []string{""},
			Locality:     []string{""},
			Organization: []string{org},
		},
		EmailAddresses: []string{""},
	}
	cert, key, err := cg.doCSR(tmpl, bits)
	if err != nil {
		return err
	}
	log.Debug("XXX Remote CA - writing out client cert and key")
	err = ioutil.WriteFile(authOptions.ClientCertPath, []byte(cert), 0644)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(authOptions.ClientKeyPath, []byte(key), 0600)
	if err != nil {
		return err
	}
	return nil
}

type CertificateSigningRequest struct {
	CertificateRequest string `json:"certificate_request,omitempty"`
	Profile            string `json:"profile"`
}
type CertificateResponse struct {
	Certificate      string `json:"certificate,omitempty"`
	CertificateChain string `json:"certificate_chain,omitempty"`
}

func (cg *CFSSLCertGenerator) doCSR(tmpl x509.CertificateRequest, bits int) (string, string, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", err
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &tmpl, privKey)
	if err != nil {
		return "", "", err
	}

	block := pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	}

	csr := &CertificateSigningRequest{
		CertificateRequest: string(pem.EncodeToMemory(&block)),
		Profile:            Profile,
	}

	buf, err := json.Marshal(csr)
	if err != nil {
		return "", "", err
	}

	csrURL := *cg.caURL
	csrURL.Path = "/api/v1/cfssl/sign"
	resp, err := cg.client.Post(csrURL.String(), "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return "", "", fmt.Errorf("Failed to send CSR to server: %s", err)
	}

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("CSR error from CA: ", string(d))
	}

	var r struct {
		// TODO - hardening based on these flags
		// Success  bool                 `json:"success"`
		Result *CertificateResponse `json:"result"`
		// Errors   []string             `json:"errors"`
		// Messages []string             `json:"messages"`
	}

	if err := json.Unmarshal(d, &r); err != nil || r.Result == nil {
		return "", "", err
	}

	// Now write out the key and cert
	privateKeyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	privateKeyEncoded := string(privateKeyBytes)

	return r.Result.Certificate, privateKeyEncoded, nil

}

// TODO This can be generalized... doesn't need to be per generator
func (cg *CFSSLCertGenerator) ReadTLSConfig(addr string, authOptions *auth.Options) (*tls.Config, error) {
	return nil, fmt.Errorf("CFSSL.ReadTLSConfig not implemented")
}

// TODO This can be generalized... doesn't need to be per generator
func (cg *CFSSLCertGenerator) ValidateCertificate(addr string, authOptions *auth.Options) (bool, error) {
	return false, fmt.Errorf("CFSSL.ValidateCertificate not implemented")
}

type InfoReq struct {
	Label   string `json:"label"`
	Profile string `json:"profile"`
}
type InfoResp struct {
	Certificate  string   `json:"certificate"`
	Usage        []string `json:"usages"`
	ExpiryString string   `json:"expiry"`
}

func (cg *CFSSLCertGenerator) doInfo() (*InfoResp, error) {
	// If we have client certs, try mutual TLS with them

	infoURL := *cg.caURL
	infoURL.Path = "/api/v1/cfssl/info"

	// TODO If a client cert was used, and mutual TLS fails, fall back to no mutual tls

	info := InfoReq{
		Profile: Profile,
	}
	buf, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	resp, err := cg.client.Post(infoURL.String(), "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return nil, fmt.Errorf("Failed to query info from remote CA: %s", err)
	}
	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error from CA: %s", string(d))
	}

	var r struct {
		// TODO - hardening based on these flags
		//Success  bool      `json:"success"`
		Result *InfoResp `json:"result"`
		//Errors   []string  `json:"errors"`
		//Messages []string  `json:"messages"`
	}

	if err := json.Unmarshal(d, &r); err != nil {
		return nil, err
	}

	return r.Result, nil
}

func BootstrapRemoteSignedCertificates(authOptions *auth.Options) error {

	/* TODO - remove this once all the kinks are sorted out
		fmt.Printf("XXX RemoteCa %s\n", authOptions.RemoteCa)                 // --ca
		fmt.Printf("XXX CaCertPath %s\n", authOptions.CaCertPath)             // --tls-ca-cert overrides default
		fmt.Printf("XXX CaPrivateKeyPath %s\n", authOptions.CaPrivateKeyPath) // --tls-ca-key overrides default
		fmt.Printf("XXX ClientCertPath %s\n", authOptions.ClientCertPath)     // --tls-client-cert overrides
		fmt.Printf("XXX ClientKeyPath %s\n", authOptions.ClientKeyPath)       // --tls-client-key overrides
		fmt.Printf("XXX ServerCertPath %s\n", authOptions.ServerCertPath)     //
		fmt.Printf("XXX ServerKeyPath %s\n", authOptions.ServerKeyPath)       //
		fmt.Printf("XXX StorePath %s\n", authOptions.StorePath)               //
		fmt.Printf("XXX ServerCertSANs %s\n", authOptions.ServerCertSANs)     //

	        Example output

		XXX CaCertPath /home/daniel/.docker/machine/certs/ca.pem
		XXX CaPrivateKeyPath /home/daniel/.docker/machine/certs/ca-key.pem
		XXX ClientCertPath /home/daniel/.docker/machine/certs/cert.pem
		XXX ClientKeyPath /home/daniel/.docker/machine/certs/key.pem
		XXX ServerCertPath /home/daniel/.docker/machine/machines/node3/server.pem
		XXX ServerKeyPath /home/daniel/.docker/machine/machines/node3/server-key.pem
		XXX StorePath /home/daniel/.docker/machine/machines/node3
		XXX ServerCertSANs []

	*/
	cg := CFSSLCertGenerator{
		authOptions: authOptions,
	}
	err := cg.Init()
	if err != nil {
		return err
	}

	if _, err := os.Stat(authOptions.CertDir); os.IsNotExist(err) {
		if err := os.MkdirAll(authOptions.CertDir, 0700); err != nil {
			return fmt.Errorf("Creating machine certificate dir failed: %s", err)
		}
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(authOptions.CaCertPath); os.IsNotExist(err) {
		log.Debug("XXX calling out to CA to figure out what it's ca.pem is")
		info, err := cg.doInfo()
		if err != nil {
			return err
		}
		log.Debug("XXX Storing ca.pem from CA")
		err = ioutil.WriteFile(authOptions.CaCertPath, []byte(info.Certificate), 0644)
		if err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("Failed to load existing %s: %s", authOptions.CaCertPath, err)
	} // Else the CA cert exists and we can just use it...

	if _, err := os.Stat(authOptions.ClientCertPath); os.IsNotExist(err) {
		caOrg := mcnutils.GetUsername()
		org := caOrg + ".<bootstrap>"
		bits := 2048
		return cg.GenerateClientCert([]string{""}, authOptions, org, bits)
	}
	return nil
}
