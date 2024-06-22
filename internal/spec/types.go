package spec

import "encoding/json"

const (
	MEDIA_TYPE_MANIFEST       = "application/vnd.oci.image.manifest.v1+json"
	//MEDIA_TYPE_MANIFEST 	  = "application/vnd.docker.distribution.manifest.v2+json"
	//MEDIA_TYPE_MANIFEST 	  = "application/vnd.nori.manifest.v2+json"
	MEDIA_TYPE_MODULE_PRIMARY = "application/vnd.nori.module.v1.tar"
	MEDIA_TYPE_MODULE_DEPS    = "application/vnd.nori.module.deps.v1.tar"
	MEDIA_TYPE_EMPTY          = "application/vnd.oci.empty.v1+json"
	MEDIA_TYPE_CONFIG         = "application/vnd.nori.config.v1+json"
	ARTIFACT_TYPE 		   	  = "application/vnd.nori.artifact.v1+json"
)

type Manifest struct {
	Schema      int               `json:"schemaVersion"`
	MediaType   string            `json:"mediaType"`
	Config      Digest            `json:"config"`
	Layers      []Digest          `json:"layers"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Digest struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Platform struct {
	Architecture *string `json:"architecture,omitempty"`
	OS           *string `json:"os,omitempty"`
}

type ImageLayout struct {
	Version string `json:"imageLayoutVersion"`
}

type Index struct {
	Schema    string   `json:"schemaVersion"`
	Manifests []Digest `json:"manifests"`
}

type ModuleInputs struct {
	Description *string `json:"description,omitempty"`
	Default     *string `json:"default,omitempty"`
}

type ModuleOutputs struct {
	Description *string `json:"description,omitempty"`
	Sensitive   *bool   `json:"sensitive,omitempty"`
}

type Config struct {
	SchemaVersion int      `json:"schemaVersion"`
	MediaType string `json:"mediaType"`
	Name string `json:"name"`
	Version string `json:"version"`
	Remote string `json:"remote"`
	Inputs map[string]ModuleInputs `json:"inputs"`
	Outputs map[string]ModuleOutputs `json:"outputs"`
}

type Tag struct {
	Host    string
	Name    string
	Namespace string
	Version string
}

func (m *Manifest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (t *Tag) String() string {
	if t.Namespace != "" {
		return t.Host + "/" + t.Namespace + "/" + t.Name + ":" + t.Version
	}
	
	return t.Host + "/" + t.Name + ":" + t.Version
}