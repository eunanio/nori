package push

import (
	"fmt"

	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/eunanio/nori/internal/spec"
)

func PushImage(tag *spec.Tag, insecure bool) error {
	if tag == nil {
		return fmt.Errorf("tag is required")
	}

	manifest, err := futils.GetTaggedManifest(tag)
	if err != nil {
		return fmt.Errorf("error getting manifest: %s", err)
	}

	creds, _ := oci.GetCredentials(tag.Host)

	reg := oci.NewRegistry(tag.Host, creds)

	err = pushLayers(manifest.Layers, tag, reg, insecure)
	if err != nil {
		return fmt.Errorf("error pushing layers: %s", err)
	}

	err = pushConfig(manifest.Config, tag, reg, insecure)
	if err != nil {
		return fmt.Errorf("error pushing config: %s", err)
	}

	err = pushManifest(manifest, tag, reg, insecure)
	if err != nil {
		return fmt.Errorf("error pushing manifest: %s", err)
	}

	fmt.Println("Image pushed successfully")
	return nil
}

func pushConfig(digest spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	fileData, err := futils.LoadBlob(digest.Digest)
	if err != nil {
		return fmt.Errorf("error loading config file: %s", err)
	}

	opts := &oci.PushBlobOptions{
		Digest:   digest,
		File:     fileData,
		Name:     tag.Name,
		Tag:      tag,
		Insecure: insecure,
	}
	err = reg.PushBlob(*opts)
	if err != nil {
		return fmt.Errorf("error pushing config: %s", err)
	}

	return nil
}

func pushLayers(layers []spec.Digest, tag *spec.Tag, reg *oci.Registry, insecure bool) error {
	for _, layer := range layers {
		fileData, err := futils.LoadBlob(layer.Digest)
		if err != nil {
			return fmt.Errorf("error loading layer file: %s", err)
		}

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
