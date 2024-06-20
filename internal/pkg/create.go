package pkg

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/hclparse"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/spec"
)

func PackageModule(packageTagFlag, packagePathFlag string) {
	// Do Stuff Here
	validatePackageFlags(packageTagFlag, packagePathFlag)
	tag, err := futils.ParseImageTag(packageTagFlag)
	if err != nil {
		fmt.Println("Error parsing tag: ", err)
		return
	}
	//imagePath := fmt.Sprintf("%s/images/%s", homepath, tag.Name)
	blobpath := paths.GetBlobDir(tag.Name, tag.Version)
	err = paths.MkDirIfNotExist(blobpath)
	if err != nil {
		fmt.Println("Error creating blob directory: ", err)
		return
	}


	moduleDigest, err := futils.CompressDir(packagePathFlag, blobpath, spec.MEDIA_TYPE_MODULE_PRIMARY,tag.Name)
	if err != nil {
		fmt.Println("Error compressing directory: ", err)
		return
	}
	moduleData, err := hclparse.ParseModuleConfig(packagePathFlag); if err != nil {
		panic(err)
	}
	dirName := paths.GetBlobDir(tag.Name, tag.Version)
	configDigest,err := generateConfig(dirName,moduleData,tag); if err != nil {
		fmt.Println("Error generating config: ", err)
	}

	err = generateManifest(*moduleDigest,*configDigest, tag); if err != nil {
		fmt.Println("Error generating manifest: ", err)
		return
	}
}

func generateManifest(digest, config spec.Digest, tag *spec.Tag) error {
	var tagName string
	if tag.Host == "" {
		tagName = fmt.Sprintf("%s:%s", tag.Name, tag.Version)
	} else {
		tagName = fmt.Sprintf("%s/%s:%s", tag.Host, tag.Name, tag.Version)
	} 

	var manifest = spec.Manifest{
		Schema:    2,
		MediaType: spec.MEDIA_TYPE_MANIFEST,
		Config:   config,
		Layers: []spec.Digest{
			digest,
		},
		Annotations: map[string]string{
			spec.ANNO_IMAGE_REF_NAME: tagName,
		},
	}

	jsonBytes, err := json.Marshal(manifest); if err != nil {
		fmt.Println("Error marshalling manifest: ", err)
		return err
	}
	manifestPath := paths.GetManifestPath(tag.Name, tag.Version)
	os.WriteFile(manifestPath,jsonBytes,os.ModePerm)
	return nil
}

func validatePackageFlags(packageFlag string, pathFlag string) {
	if packageFlag == "" {
		fmt.Println("Tag is required")
		os.Exit(1)
	}
	if pathFlag == "" {
		fmt.Println("Path is required")
		os.Exit(1)
	}
}

func generateConfig(blobPath string, data *hclparse.ModuleConfig, tag *spec.Tag) (*spec.Digest,error){
	var inputs = make(map[string]spec.ModuleInputs)
	var outputs = make(map[string]spec.ModuleOutputs)
	for _, value := range data.Inputs {
		var input = spec.ModuleInputs{
			Description: value.Description,
			Default: value.Default,
		}
		inputs[value.Name] = input
	}

	for _, value := range data.Outputs {
		var output = spec.ModuleOutputs{
			Description: value.Description,
			Sensitive: value.Sensitive,
		}
		outputs[value.Name] = output
	}

	config := spec.Config{
		SchemaVersion: 1,
		MediaType: spec.MEDIA_TYPE_CONFIG,
		Name: tag.Name,
		Version: tag.Version,
		Remote: tag.Host,
		Inputs: inputs,
		Outputs: outputs,
	}

	jsonBytes, err := json.Marshal(config); if err != nil {
		fmt.Println("Error marshalling config: ", err)
		return nil,err
	}

	emptyDigest,err := futils.WriteBlob(jsonBytes,blobPath,spec.MEDIA_TYPE_CONFIG); if err != nil {
		fmt.Println("Error compressing empty json: ", err)
		return nil,err
	}
	
	return emptyDigest,nil
}