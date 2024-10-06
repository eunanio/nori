package paths

import (
	"fmt"
	"os"
)

const (
	TLP_CREDS_PATH   = "%s/credentials.json"
	TLP_BLOB_PATH_V2 = "%s/images/blobs/%s"
	TLP_MAP_PATH     = "%s/images/index.json"
	TLP_BLOB_FILE    = "%s/images/blobs/%s/%s"
	TLP_HOME_DIR     = "%s/.nori"
	TLP_IMAGE_DIR    = "%s/images/%s/%s"
	TLP_CONFIG_FILE  = "%s/config.json"
	TLP_RELEASE_PATH = "%s/releases/%s"
	TLP_RELEASE_FILE = "%s/releases/releases.json"
	TLP_STATE_PATH   = "%s/state/%s"
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

func GetBlobDirV2(sha string) string {
	homePath := GetOrCreateHomePath()
	shard1 := sha[:2]
	return fmt.Sprintf(TLP_BLOB_PATH_V2, homePath, shard1)
}

func GetBlobPathV2(sha string) string {
	homePath := GetOrCreateHomePath()
	shard1 := sha[:2]
	return fmt.Sprintf(TLP_BLOB_FILE, homePath, shard1, sha)
}

func GetImagePath(name, version string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_IMAGE_DIR, homePath, name, version)
}

func GetCredsPath() string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_CREDS_PATH, homePath)
}

func GetConfigPath() string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_CONFIG_FILE, homePath)
}

func GetReleasePath(name string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_RELEASE_PATH, homePath, name)
}

func GetStatePath(name string) string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_STATE_PATH, homePath, name)
}

func GetReleaseFilePath() string {
	homePath := GetOrCreateHomePath()
	val, ok := os.LookupEnv("RELEASE_PATH")
	if ok {
		return val
	}
	return fmt.Sprintf(TLP_RELEASE_FILE, homePath)
}

func GetModuleMapPath() string {
	homePath := GetOrCreateHomePath()
	return fmt.Sprintf(TLP_MAP_PATH, homePath)
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

func GetHome() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting user home directory: ", err)
		return ""
	}
	homePath := fmt.Sprintf(TLP_HOME_DIR, homeDir)
	return homePath
}
