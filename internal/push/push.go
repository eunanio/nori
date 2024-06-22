package push

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanhardy/nori/internal/e"
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/oci"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/spec"
)

func PushImage(tag *spec.Tag, insecure bool) {
	// Do Stuff Here
	if tag == nil {
		panic("Error: Invalid tag")
	}

	configPath := paths.GetOrCreateHomePath()
	manifestPath := paths.GetManifestPath(tag.Name, tag.Version)
	var manifest spec.Manifest
	if !futils.FileExists(manifestPath) {
		panic("Error: Manifest not found")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		panic("Error reading manifest: " + err.Error())
	}

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		panic("Error unmarshalling manifest: " + err.Error())
	}
	creds, _ := oci.GetCredentials(tag.Host)

	reg := oci.NewRegistry(tag.Host, creds)

	err = pushLayers(configPath, manifest.Layers, tag, reg, insecure)
	e.Resolve(err, "Error pushing layers")

	err = pushConfig(configPath, manifest.Config, tag, reg, insecure)
	e.Resolve(err, "Error pushing config")

	err = pushManifest(manifest, tag, reg, insecure)
	e.Resolve(err, "Error pushing manifest")

	fmt.Println("Image pushed successfully")
}

func pushConfig(cp string, digest spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	fileData, err := futils.LoadBlobContent(cp, digest.Digest, tag)
	e.Resolve(err, "Error loading config file")

	opts := &oci.PushBlobOptions{
		Digest: digest,
		File:   fileData,
		Name:   tag.Name,
		Tag:   	tag,
		Insecure: insecure,
	}
	err = reg.PushBlob(*opts)
	e.Fatal(err, "Error pushing config file")

	return nil
}

func pushLayers(cp string, layers []spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	for _, layer := range layers {
		fileData, err := futils.LoadBlobContent(cp, layer.Digest, tag)
		e.Resolve(err, "Error loading layer file")

		opts := &oci.PushBlobOptions{
			Digest: layer,
			File:   fileData,
			Name:   tag.Name,
			Tag:  	tag,
			Insecure: insecure,
		}

		err = reg.PushBlob(*opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func pushManifest(manifest spec.Manifest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	opts := &oci.PushManifestOptions{
		Manifest: manifest,
		Tag:      tag,
		Insecure: insecure,
	}

	err := reg.PushManifest(*opts)
	e.Resolve(err, "Error pushing manifest")

	return nil
}