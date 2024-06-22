package pull

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

func PullImage(tag *spec.Tag, export bool, ctxPath string) (*spec.Manifest, *spec.Config) {
	// if tag.Host == "" {
	// 	panic("Error: Tag must be remote")
	// }
	configPath := paths.GetOrCreateHomePath()
	manifestPath := paths.GetManifestPath(tag.Name, tag.Version)
	err := paths.MkDirIfNotExist(paths.GetBlobDir(tag.Name, tag.Version))
	e.Resolve(err, "Error creating image directory")

	fmt.Printf("Pulling image: %s...\n", tag.String())
	creds, _ := oci.GetCredentials(tag.Host)
	//e.Resolve(err, "Error getting credentials")

	reg := oci.NewRegistry(tag.Host, creds)

	manifest, err := reg.PullManifest(tag)
	e.Fatal(err, "Error pulling manifest")

	manifestBytes, err := manifest.Marshal()
	e.Resolve(err, "Error marshalling manifest")

	os.WriteFile(manifestPath, manifestBytes, 0644)
	pullLayers(reg, manifest, tag, configPath)
	config, err := pullConfig(reg, manifest, tag, configPath)
	e.Fatal(err, "Error pulling config")

	fmt.Println("Image pulled successfully")
	if export {
		fmt.Printf("Unpacking module into `%s/.nori/%s`...\n", ctxPath, tag.Name)
		createAndExport(manifest, tag, configPath, ctxPath)
	}

	return manifest, config
}

func pullLayers(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag, cp string) {

	for _, layer := range manifest.Layers {
		sha := layer.Digest[7:]
		layerPath := paths.GetBlobPath(tag.Name, tag.Version, sha)
		if futils.FileExists(layerPath) {
			fmt.Printf("%s: already exists\n", sha[:24])
			continue
		}
		fmt.Printf("%s: Pulling\n", layer.Digest[:24])
		pullOpts := oci.PullBlobOptions{
			Name:   tag.Name,
			Digest: layer,
			Tag:   tag,
		}

		layerData, err := reg.PullBlob(pullOpts)
		e.Resolve(err, "Error pulling layer")
		blobOpts := futils.BlobWriter{
			ConfigPath:  cp,
			RepoName:    tag.Name,
			RepoVersion: tag.Version,
			Data:        layerData,
			Digest:      layer.Digest,
		}

		err = futils.WriteBlobContent(blobOpts)
		e.Fatal(err, "Error writing layer")
	}
}

func pullConfig(reg *oci.Registry, manifest *spec.Manifest, tag *spec.Tag, cp string) (config *spec.Config, err error) {
	sha := manifest.Config.Digest[7:]
	configPath := paths.GetBlobPath(tag.Name, tag.Version, sha)
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
	fmt.Printf("%s: Pulling\n", manifest.Config.Digest[:24])
	pullOpts := oci.PullBlobOptions{
		Name:   tag.Name,
		Digest: manifest.Config,
		Tag:   tag,
	}

	configData, err := reg.PullBlob(pullOpts)
	if err != nil {
		return nil, err
	}

	blobOpts := futils.BlobWriter{
		ConfigPath:  cp,
		RepoName:    tag.Name,
		RepoVersion: tag.Version,
		Data:        configData,
		Digest:      manifest.Config.Digest,
	}

	err = futils.WriteBlobContent(blobOpts)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(configData, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func createAndExport(manifest *spec.Manifest, tag *spec.Tag, cp string, ctxPath string) {
	path := fmt.Sprintf("%s/.nori", ctxPath)
	paths.MkDirIfNotExist(path)
	for _, layer := range manifest.Layers {
		sha := layer.Digest[7:]
		layerPath := paths.GetBlobPath(tag.Name, tag.Version, sha)
		if !futils.FileExists(layerPath) {
			panic("Error: Layer not found")
		}
		if layer.MediaType == spec.MEDIA_TYPE_MODULE_PRIMARY {
			layerData, err := futils.LoadBlobContent(cp, layer.Digest, tag)
			e.Resolve(err, "Error loading layer")
			futils.DecompressModule(layerData, path)
		} else {
			fmt.Printf("%s:Skipping\n", sha[:12])
		}
	}

}