package pull

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/e"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/eunanio/nori/internal/paths"
	"github.com/eunanio/nori/internal/spec"
)

func PullImage(tag *spec.Tag, export bool, ctxPath string) (*spec.Manifest, *spec.Config) {

	fmt.Printf("Pulling image: %s...\n", tag.String())
	creds, _ := oci.GetCredentials(tag.Host)
	//e.Resolve(err, "Error getting credentials")
	manifest, err := futils.GetTaggedManifest(tag)
	if err != nil {
		e.Fatal(err, "Error getting manifest")
	}
	reg := oci.NewRegistry(tag.Host, creds)
	if manifest == nil {
		manifest, err := reg.PullManifest(tag)
		e.Fatal(err, "Error pulling manifest")

		manifestBytes, err := manifest.Marshal()
		e.Resolve(err, "Error marshalling manifest")
		futils.WriteBlob(manifestBytes, spec.MEDIA_TYPE_MANIFEST)
	}
	// os.WriteFile(manifestPath, manifestBytes, 0644)
	pullLayers(reg, manifest, tag)
	config, err := PullConfig(reg, manifest, tag)
	e.Fatal(err, "Error pulling config")

	fmt.Println("Image pulled successfully")
	if export {
		fmt.Printf("Unpacking module into `%s/.nori/%s`...\n", ctxPath, tag.Name)
		createAndExport(manifest, ctxPath)
	}

	return manifest, config
}

func pullLayers(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag) {

	for _, layer := range manifest.Layers {
		sha := layer.Digest[7:]
		layerPath := paths.GetBlobPathV2(sha)
		if futils.FileExists(layerPath) {
			fmt.Printf("%s: already exists\n", sha[:24])
			continue
		}
		fmt.Printf("%s: Pulling\n", layer.Digest[:24])
		pullOpts := oci.PullBlobOptions{
			Name:   tag.Name,
			Digest: layer,
			Tag:    tag,
		}

		layerData, err := reg.PullBlob(pullOpts)
		e.Resolve(err, "Error pulling layer")

		_, err = futils.WriteBlob(layerData, spec.MEDIA_TYPE_MODULE_PRIMARY)
		e.Fatal(err, "Error writing layer")
	}
}

func PullConfig(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag) (config *spec.Config, err error) {
	sha := manifest.Config.Digest[7:]
	configPath := paths.GetBlobPathV2(sha)
	if futils.FileExists(configPath) {
		fmt.Printf("%s: already exists\n", sha[:24])
		configBytes, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, err
		}

		return config, nil
	}
	sha = manifest.Config.Digest[7:]
	fmt.Printf("%s: Pulling\n", sha[:24])
	pullOpts := oci.PullBlobOptions{
		Name:   tag.Name,
		Digest: manifest.Config,
		Tag:    tag,
	}

	configData, err := reg.PullBlob(pullOpts)
	if err != nil {
		return nil, err
	}

	_, err = futils.WriteBlob(configData, spec.MEDIA_TYPE_CONFIG)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(configData, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func createAndExport(manifest *spec.Manifest, ctxPath string) {
	path := fmt.Sprintf("%s/.nori", ctxPath)
	paths.MkDirIfNotExist(path)
	for _, layer := range manifest.Layers {
		sha := layer.Digest[7:]
		layerPath := paths.GetBlobPathV2(sha)
		if !futils.FileExists(layerPath) {
			panic("Error: Layer not found")
		}
		if layer.MediaType == spec.MEDIA_TYPE_MODULE_PRIMARY {
			layerData, err := futils.LoadBlob(layer.Digest)
			if err != nil {
				panic(err)
			}
			err = futils.DecompressModule(layerData, path)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Printf("%s: skipping\n", sha[:24])
		}
	}

}
