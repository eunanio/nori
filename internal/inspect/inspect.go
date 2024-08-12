package inspect

import (
	"encoding/json"
	"fmt"

	"github.com/eunanio/nori/internal/e"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/eunanio/nori/internal/pull"
	"github.com/eunanio/nori/internal/spec"
)

func GetImageInfo(tag *spec.Tag) {
	creds, _ := oci.GetCredentials(tag.Host)
	//e.Resolve(err, "Error getting credentials")
	manifest, err := futils.GetTaggedManifest(tag)
	if err != nil {
		e.Fatal(err, "Unable to get a Package")
	}
	reg := oci.NewRegistry(tag.Host, creds)
	if manifest == nil {
		manifest, err = reg.PullManifest(tag)
		if err != nil {
			panic("error inspecting manifest")
		}
	}

	config, err := pull.PullConfig(reg, manifest, tag)
	if err != nil {
		panic("error inspecting config: " + err.Error())
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic("error marshalling config")
	}
	fmt.Println(string(data))

}
