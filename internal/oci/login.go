package oci

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/paths"
)

type RegistryCreds struct {
	Credentials map[string]string `json:"credentials,omitempty"`
}


func Login(url, username, password string) error {
	credsFile := paths.GetCredsPath()
	var registryCreds RegistryCreds
	//base64 encode the username and password
	userpass := fmt.Sprintf("%s:%s", username, password)
	encoded := base64.StdEncoding.EncodeToString([]byte(userpass))

	if futils.FileExists(credsFile) {
		fileBytes, err := os.ReadFile(credsFile); if err != nil {
			panic(fmt.Errorf("error reading creds file: %s", err))
		}
		err = json.Unmarshal(fileBytes, &registryCreds); if err != nil {
			panic(fmt.Errorf("error unmarshalling creds file: %s", err))
		}	
	}

	if registryCreds.Credentials == nil {
		registryCreds.Credentials = make(map[string]string)
		registryCreds.Credentials[url] = encoded
	} else {
		registryCreds.Credentials[url] = encoded
	}

	credsJson, err := json.Marshal(registryCreds); if err != nil {
		panic(fmt.Errorf("error marshalling creds file: %s", err))
	}

	err = os.WriteFile(credsFile, credsJson, 0644); if err != nil {
		panic(fmt.Errorf("error writing creds file: %s", err))
	}

	fmt.Println("Login Successful")
	return nil
}

func GetCredentials(url string) (string, error) {
	credsFile := paths.GetCredsPath()
	var registryCreds RegistryCreds
	if futils.FileExists(credsFile) {
		fileBytes, err := os.ReadFile(credsFile); if err != nil {
			return "", fmt.Errorf("error reading creds file: %s", err)
		}
		err = json.Unmarshal(fileBytes, &registryCreds); if err != nil {
			return "", fmt.Errorf("error unmarshalling creds file: %s", err)
		}
	} else {
		return "", fmt.Errorf("no credentials found")
	}

	if creds, ok := registryCreds.Credentials[url]; ok {
		return creds, nil
	} else {
		return "", fmt.Errorf("no credentials found")
	}
}
