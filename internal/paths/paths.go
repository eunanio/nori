package paths

import (
	"fmt"
	"os"
)

const (
	TLP_MANIFEST_PATH = "%s/images/%s/%s/manifest.json"
	TLP_CREDS_PATH	  = "%s/credentials.json"
	TLP_BLOB_PATH     = "%s/images/%s/%s/blobs/sha256"
	TLP_BLOB_FILE     = "%s/images/%s/%s/blobs/sha256/%s"
	TLP_HOME_DIR      = "%s/.nori"
	TLP_IMAGE_DIR     = "%s/images/%s/%s"
	TLP_CONFIG_FILE   = "%s/config.json"
	TLP_RELEASE_PATH  = "%s/releases/%s"
)

func GetOrCreateHomePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting user home directory: ", err)
		return ""
	}
	configPath := fmt.Sprintf(TLP_HOME_DIR, homeDir)
	MkDirIfNotExist(configPath)
	MkDirIfNotExist(fmt.Sprintf("%s/images", configPath))
	MkDirIfNotExist(fmt.Sprintf("%s/releases", configPath))
	return configPath
}

func GetManifestPath(name, version string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_MANIFEST_PATH, homePath, name, version)
}

func GetBlobPath(name, version, sha string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_BLOB_FILE, homePath, name, version, sha)
}

func GetBlobDir(name, version string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_BLOB_PATH, homePath, name, version)
}

func GetImagePath(name, version string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_IMAGE_DIR, homePath, name, version)
}

func GetCredsPath() string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_CREDS_PATH, homePath)
}

func GetConfigPath() string{
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_CONFIG_FILE,homePath)
}

func GetReleasePath(name string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_RELEASE_PATH, homePath, name)
}

func MkDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

