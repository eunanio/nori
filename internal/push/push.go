package push

import (
	"fmt"

	"github.com/eunanio/nori/internal/e"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/eunanio/nori/internal/spec"
)

func PushImage(tag *spec.Tag, insecure bool) {
	// Do Stuff Here
	if tag == nil {
		panic("Error: Invalid tag")
	}

	manifest, err := futils.GetTaggedManifest(tag)
	if err != nil {
		e.Fatal(err, "Error getting manifest")
	}

	creds, _ := oci.GetCredentials(tag.Host)

	reg := oci.NewRegistry(tag.Host, creds)

	err = pushLayers(manifest.Layers, tag, reg, insecure)
	e.Resolve(err, "Error pushing layers")

	err = pushConfig(manifest.Config, tag, reg, insecure)
	e.Resolve(err, "Error pushing config")

	err = pushManifest(manifest, tag, reg, insecure)
	e.Resolve(err, "Error pushing manifest")

	fmt.Println("Image pushed successfully")
}

func pushConfig(digest spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	fileData, err := futils.LoadBlob(digest.Digest)
	e.Resolve(err, "Error loading config file")

	opts := &oci.PushBlobOptions{
		Digest:   digest,
		File:     fileData,
		Name:     tag.Name,
		Tag:      tag,
		Insecure: insecure,
	}
	err = reg.PushBlob(*opts)
	e.Fatal(err, "Error pushing config file")

	return nil
}

func pushLayers(layers []spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	for _, layer := range layers {
		fileData, err := futils.LoadBlob(layer.Digest)
		e.Resolve(err, "Error loading layer file")

		opts := &oci.PushBlobOptions{
			Digest:   layer,
			File:     fileData,
			Name:     tag.Name,
			Tag:      tag,
			Insecure: insecure,
		}

		err = reg.PushBlob(*opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func pushManifest(manifest *spec.Manifest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	opts := &oci.PushManifestOptions{
		Manifest: manifest,
		Tag:      tag,
		Insecure: insecure,
	}

	err := reg.PushManifest(*opts)
	if err != nil {
		return fmt.Errorf("error pushing manifest: %s", err)
	}

	return nil
}
