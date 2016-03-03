package ssh

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/log"
)

var (
	ErrKeyGeneration     = errors.New("Unable to generate key")
	ErrValidation        = errors.New("Unable to validate key")
	ErrPublicKey         = errors.New("Unable to convert public key")
	ErrUnableToWriteFile = errors.New("Unable to write file")

	Store  store.Store
	Prefix = ""
)

type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

// NewKeyPair generates a new SSH keypair
// This will return a private & public key encoded as DER.
func NewKeyPair() (keyPair *KeyPair, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, ErrKeyGeneration
	}

	if err := priv.Validate(); err != nil {
		return nil, ErrValidation
	}

	privDer := x509.MarshalPKCS1PrivateKey(priv)

	pubSSH, err := gossh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, ErrPublicKey
	}

	return &KeyPair{
		PrivateKey: privDer,
		PublicKey:  gossh.MarshalAuthorizedKey(pubSSH),
	}, nil
}

// WriteToFile writes keypair to files
func (kp *KeyPair) WriteToFile(privateKeyPath string, publicKeyPath string) error {
	if err := setStore(privateKeyPath); err != nil {
		return err
	}
	files := []struct {
		File  string
		Type  string
		Value []byte
	}{
		{
			File:  privateKeyPath,
			Value: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Headers: nil, Bytes: kp.PrivateKey}),
		},
		{
			File:  publicKeyPath,
			Value: kp.PublicKey,
		},
	}

	for _, v := range files {
		err := writeFile(v.File, v.Value, 0, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeFile(filename string, data []byte, flag int, mode os.FileMode) error {
	path := strings.TrimPrefix(filename, Prefix)

	key := filepath.Join("/", MachinePrefix, path)
	log.Debugf("XXX KV SSH Write -> %s", key)
	err := Store.Put(key, data, nil)
	log.Debugf("XXX err: %s", err)
	return err
}

// Fingerprint calculates the fingerprint of the public key
func (kp *KeyPair) Fingerprint() string {
	b, _ := base64.StdEncoding.DecodeString(string(kp.PublicKey))
	h := md5.New()

	io.WriteString(h, string(b))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// HACK!!!
const MachinePrefix = "machine/v0"

func init() {
	etcd.Register()
	//consul.Register()
	//zookeeper.Register()
	//boltdb.Register()
}
func setStore(path string) error {
	if Store != nil {
		return nil
	}
	kvurl, err := url.Parse(path) // XXX Prefix is going to be wrong!
	if err != nil {
		return fmt.Errorf("Malformed store path: %s %s", path, err)
	}
	switch kvurl.Scheme {
	case "etcd":
		// TODO - figure out how to get TLS support in here...
		kvStore, err := libkv.NewStore(
			store.ETCD,
			[]string{kvurl.Host},
			&store.Config{
				ConnectionTimeout: 10 * time.Second,
			},
		)
		if err != nil {
			return err
		}
		Store = kvStore
		// TODO other KV store types
	default:
		return fmt.Errorf("Unsupporetd KV store type: %s", kvurl.Scheme)
	}
	Prefix = fmt.Sprintf("%s://%s", kvurl.Scheme, kvurl.Host)
	// TODO - doesn't handle local paths!!!
	return nil
}
func Exists(filename string) bool {
	path := strings.TrimPrefix(filename, Prefix)

	key := filepath.Join("/", MachinePrefix, path)
	log.Debugf("XXX KV Exists -> %s", key)
	exists, err := Store.Exists(key)
	if err != nil {
		// TODO log a better message on other errors
		log.Errorf("KV lookup failure on %s: %s", filename, err)
		return false
	}
	log.Debugf("XXX Exists: %v", exists)
	return exists
}

// GenerateSSHKey generates SSH keypair based on path of the private key
// The public key would be generated to the same path with ".pub" added
func GenerateSSHKey(path string) error {
	log.Debugf("XXX GenerateSSHKey -> %s", path)
	if err := setStore(path); err != nil {
		return err
	}

	if !Exists(path) {

		kp, err := NewKeyPair()
		if err != nil {
			return fmt.Errorf("Error generating key pair: %s", err)
		}

		if err := kp.WriteToFile(path, fmt.Sprintf("%s.pub", path)); err != nil {
			return fmt.Errorf("Error writing keys to file(s): %s", err)
		}
	}

	return nil
}
