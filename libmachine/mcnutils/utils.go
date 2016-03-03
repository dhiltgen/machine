package mcnutils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/machine/libmachine/log"
)

const MachinePrefix = "machine/v0"

func init() {
	etcd.Register()
	//consul.Register()
	//zookeeper.Register()
	//boltdb.Register()
}

// GetHomeDir returns the home directory
// TODO: Having this here just strikes me as dangerous, but some of the drivers
// depend on it ;_;
func GetHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func GetUsername() string {
	u := "unknown"
	osUser := ""

	switch runtime.GOOS {
	case "darwin", "linux":
		osUser = os.Getenv("USER")
	case "windows":
		osUser = os.Getenv("USERNAME")
	}

	if osUser != "" {
		u = osUser
	}

	return u
}

func CopyFile(src, dst string) error {
	log.Debugf("XXX CopyFile(%s, %s)", src, dst)

	// TODO - handle permissions sanely

	data, err := ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, data)
}

func ReadFile(filename string) ([]byte, error) {
	// XXX this is not a great model... but will work for now...

	// TOTAL hack - need to fix this
	if strings.HasPrefix(filename, "etcd:/") && string(filename[6]) != "/" {
		filename = "etcd:/" + filename[5:]
	}

	storeURL, err := url.Parse(filename)
	if err == nil {
		// The scheme will be blank on unix paths, might be a drive letter (single char)
		// or a multi-character scheme that libkv will hopefully handle
		if len(storeURL.Scheme) > 1 {
			var kvStore store.Store
			switch storeURL.Scheme {
			case "etcd":
				// TODO - figure out how to get TLS support in here...
				kvStore, err = libkv.NewStore(
					store.ETCD,
					[]string{storeURL.Host},
					&store.Config{
						ConnectionTimeout: 10 * time.Second,
					},
				)
			default:
				return nil, fmt.Errorf("Unsupporetd KV store type: %s", storeURL.Scheme)
			}

			prefix := fmt.Sprintf("%s://%s", storeURL.Scheme, storeURL.Host)
			path := strings.TrimPrefix(filename, prefix)
			key := filepath.Join("/", MachinePrefix, path)
			log.Debugf("XXX mcnutils KV Read -> %s", key)
			kvpair, err := kvStore.Get(key)
			if err != nil {
				log.Error(err)
				return nil, err
			}
			return kvpair.Value, nil

		}
	}
	return ioutil.ReadFile(filename)

}
func WriteFile(filename string, data []byte) error {
	// XXX this is not a great model... but will work for now...
	// TOTAL hack - need to fix this
	if strings.HasPrefix(filename, "etcd:/") && string(filename[6]) != "/" {
		filename = "etcd:/" + filename[5:]
	}
	storeURL, err := url.Parse(filename)
	if err == nil {
		// The scheme will be blank on unix paths, might be a drive letter (single char)
		// or a multi-character scheme that libkv will hopefully handle
		if len(storeURL.Scheme) > 1 {
			var kvStore store.Store
			switch storeURL.Scheme {
			case "etcd":
				// TODO - figure out how to get TLS support in here...
				kvStore, err = libkv.NewStore(
					store.ETCD,
					[]string{storeURL.Host},
					&store.Config{
						ConnectionTimeout: 10 * time.Second,
					},
				)
			default:
				return fmt.Errorf("Unsupporetd KV store type: %s", storeURL.Scheme)
			}

			prefix := fmt.Sprintf("%s://%s", storeURL.Scheme, storeURL.Host)
			path := strings.TrimPrefix(filename, prefix)
			key := filepath.Join("/", MachinePrefix, path)
			log.Debugf("XXX mcnutils KV Write -> %s", key)
			err := kvStore.Put(key, data, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return nil

		}
	}
	return ioutil.WriteFile(filename, data, 0600)
}

func Join(base string, elem ...string) string {
	// TOTAL hack - need to fix this
	if strings.HasPrefix(base, "etcd:/") && string(base[6]) != "/" {
		base = "etcd:/" + base[5:]
	}
	baseURL, err := url.Parse(base)
	if err == nil {
		if len(baseURL.Scheme) > 1 {
			baseURL.Path = filepath.Join(append([]string{baseURL.Path}, elem...)...)
			return baseURL.String()
		}
	}
	ret := filepath.Join(append([]string{base}, elem...)...)
	return ret
}

func WaitForSpecificOrError(f func() (bool, error), maxAttempts int, waitInterval time.Duration) error {
	for i := 0; i < maxAttempts; i++ {
		stop, err := f()
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		time.Sleep(waitInterval)
	}
	return fmt.Errorf("Maximum number of retries (%d) exceeded", maxAttempts)
}

func WaitForSpecific(f func() bool, maxAttempts int, waitInterval time.Duration) error {
	return WaitForSpecificOrError(func() (bool, error) {
		return f(), nil
	}, maxAttempts, waitInterval)
}

func WaitFor(f func() bool) error {
	return WaitForSpecific(f, 60, 3*time.Second)
}

// TruncateID returns a shorten id
// Following two functions are from github.com/docker/docker/utils module. It
// was way overkill to include the whole module, so we just have these bits
// that we're using here.
func TruncateID(id string) string {
	shortLen := 12
	if len(id) < shortLen {
		shortLen = len(id)
	}
	return id[:shortLen]
}

// GenerateRandomID returns an unique id
func GenerateRandomID() string {
	for {
		id := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, id); err != nil {
			panic(err) // This shouldn't happen
		}
		value := hex.EncodeToString(id)
		// if we try to parse the truncated for as an int and we don't have
		// an error then the value is all numeric and causes issues when
		// used as a hostname. ref #3869
		if _, err := strconv.ParseInt(TruncateID(value), 10, 64); err == nil {
			continue
		}
		return value
	}
}
