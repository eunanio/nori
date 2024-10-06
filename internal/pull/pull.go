package pull

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/eunanio/nori/internal/paths"
	"github.com/eunanio/nori/internal/spec"
)

func PullImage(tag *spec.Tag, export bool, ctxPath string) (*spec.Manifest, *spec.Config, error) {

	creds, _ := oci.GetCredentials(tag.Host)
	manifest, err := futils.GetTaggedManifest(tag)
	if err != nil {
		return nil, nil, err
	}

	reg := oci.NewRegistry(tag.Host, creds)
	console.Debug(fmt.Sprint(manifest))
	if manifest == nil {
		console.Println("Pulling image...")
		manifest, err := reg.PullManifest(tag)
		if err != nil {
			return nil, nil, fmt.Errorf("error pulling manifest: %s", err)
		}

		manifestBytes, err := json.Marshal(manifest)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling manifest: %s", err)
		}

		_, err = futils.WriteBlob(manifestBytes, spec.MEDIA_TYPE_MANIFEST)
		if err != nil {
			return nil, nil, fmt.Errorf("error writing manifest: %s", err)
		}
		console.Debug(fmt.Sprint(manifest))
		pullLayers(reg, manifest, tag)
	}
	console.Debug(fmt.Sprint(manifest))

	config, err := PullConfig(reg, manifest, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("error pulling config: %s", err)
	}

	if export {
		console.Debug(fmt.Sprintf("Unpacking module into `%s/.nori/%s`...\n", ctxPath, tag.Name))
		createAndExport(manifest, ctxPath)
	}

	return manifest, config, nil
}

func pullLayers(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag) error {
	console.Debug("layer manifest: " + fmt.Sprint(manifest))
	for _, layer := range manifest.Layers {
		sha := layer.Digest[7:]
		layerPath := paths.GetBlobPathV2(sha)
		if futils.FileExists(layerPath) {
			console.Debug(fmt.Sprintf("%s: already exists\n", sha[:24]))
			continue
		}

		console.Debug(fmt.Sprintf("%s: Pulling\n", sha[:24]))

		pullOpts := oci.PullBlobOptions{
			Name:   tag.Name,
			Digest: layer,
			Tag:    tag,
		}

		layerData, err := reg.PullBlob(pullOpts)
		if err != nil {
			return fmt.Errorf("error pulling layer: %s", err)
		}

		_, err = futils.WriteBlob(layerData, spec.MEDIA_TYPE_MODULE_PRIMARY)
		if err != nil {
			return fmt.Errorf("error writing layer: %s", err)
		}
	}

	return nil
}

func PullConfig(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag) (config *spec.Config, err error) {
	sha := manifest.Config.Digest[7:]
	configPath := paths.GetBlobPathV2(sha)
	if futils.FileExists(configPath) {
		console.Debug(fmt.Sprintf("%s: already exists\n", sha[:24]))
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
	console.Debug(fmt.Sprintf("%s: Pulling\n", sha[:24]))
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
			console.Println(fmt.Sprintf("Skipping: %s\n", sha[:24]))
		}
	}

}
