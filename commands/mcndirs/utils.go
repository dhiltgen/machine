package mcndirs

import (
	"net/url"
	"os"
	"path/filepath"

	"github.com/docker/machine/libmachine/mcnutils"
)

var (
	BaseDir = os.Getenv("MACHINE_STORAGE_PATH")
)

func GetBaseDir() string {
	if BaseDir == "" {
		BaseDir = filepath.Join(mcnutils.GetHomeDir(), ".docker", "machine")
	}
	return BaseDir
}

func GetMachineDir() string {
	// Might be a URL
	base := GetBaseDir()
	baseURL, err := url.Parse(base)
	if err != nil {
		return filepath.Join(base, "machines")
	}
	baseURL.Path = filepath.Join(baseURL.Path, "machines")
	return baseURL.String()
}

func GetMachineCertDir() string {
	// Might be a URL
	base := GetBaseDir()
	baseURL, err := url.Parse(base)
	if err != nil {
		return filepath.Join(base, "certs")
	}
	baseURL.Path = filepath.Join(baseURL.Path, "certs")
	return baseURL.String()
}
