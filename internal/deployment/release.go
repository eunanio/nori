package deployment

import (
	"fmt"
	"os"

	"encoding/json"

	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/paths"
)

type ReleaseState struct {
	Releases map[string]Release `json:"releases"`
}

type Release struct {
	Id     string `json:"id"`
	Tag    string `json:"tag"`
	Values string `json:"values"`
}

func UpdateOrCreateReleaseState(release Release) error {
	state, err := loadRelease()
	if err != nil {
		return err
	}

	if state.Releases == nil {
		state.Releases = make(map[string]Release)
	}

	state.Releases[release.Id] = release

	stateBytes, err := json.Marshal(state)
	if err != nil {
		return err
	}

	err = os.WriteFile(paths.GetReleaseFilePath(), stateBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func RemoveReleaseFromState(releaseId string) error {
	state, err := loadRelease()
	if err != nil {
		return err
	}

	delete(state.Releases, releaseId)
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("error marshalling state: %s", err.Error())
	}

	err = os.WriteFile(paths.GetReleaseFilePath(), stateBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func ListReleases() {
	state, err := loadRelease()
	if err != nil {
		fmt.Println("Error loading release state: ", err)
		return
	}

	if state.Releases == nil || len(state.Releases) == 0 {
		fmt.Println("No releases found")
		return
	}

	fmt.Println("ID\t\t\t\t\t\t|\t Tag") // I know this is cursed

	for _, release := range state.Releases {
		fmt.Println(release.Id+"\t|\t", release.Tag)
	}
}

func loadRelease() (ReleaseState, error) {
	state := ReleaseState{}
	path := paths.GetReleaseFilePath()
	if futils.FileExists(path) {
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return state, err
		}

		err = json.Unmarshal(fileBytes, &state)
		if err != nil {
			return state, err
		}

		return state, nil
	}
	return state, nil
}
