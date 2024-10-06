package spec

import "encoding/json"

const (
	MEDIA_TYPE_MANIFEST = "application/vnd.oci.image.manifest.v1+json"
	//MEDIA_TYPE_MANIFEST 	  = "application/vnd.docker.distribution.manifest.v2+json"
	//MEDIA_TYPE_MANIFEST 	  = "application/vnd.nori.manifest.v2+json"
	//	MEDIA_TYPE_MODULE_PRIMARY = "application/vnd.nori.module.v1.tar"
	MEDAI_TYPE_LAYER          = "application/vnd.nori.layer.v1.tar+gzip"
	MEDIA_TYPE_MODULE_PRIMARY = "application/vnd.oci.image.layer.v1.tar+gzip"
	MEDIA_TYPE_EMPTY          = "application/vnd.oci.empty.v1+json"
	MEDIA_TYPE_CONFIG         = "application/vnd.nori.module.config.v1+json"
	ARTIFACT_TYPE             = "application/vnd.nori.module.manifest.v1+json"
)

type Manifest struct {
	Schema       int               `json:"schemaVersion"`
	MediaType    string            `json:"mediaType"`
	ArtifactType string            `json:"artifactType"`
	Config       Digest            `json:"config"`
	Layers       []Digest          `json:"layers"`
	Annotations  map[string]string `json:"annotations,omitempty"`
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
	Description *string     `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Sensitive   *bool       `json:"sensitive,omitempty"`
}

type ModuleOutputs struct {
	Description *string `json:"description,omitempty"`
	Sensitive   *bool   `json:"sensitive,omitempty"`
}

type Config struct {
	SchemaVersion int                      `json:"schemaVersion"`
	MediaType     string                   `json:"mediaType"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Remote        string                   `json:"remote,omitempty"`
	Resources     map[string]int           `json:"resources,omitempty"`
	Inputs        map[string]ModuleInputs  `json:"inputs,omitempty"`
	Outputs       map[string]ModuleOutputs `json:"outputs,omitempty"`
}

type Tag struct {
	Host      string
	Name      string
	Namespace string
	Version   string
}

func (m *Manifest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (t *Tag) String() string {
	if t.Namespace != "" {
		return t.Host + "/" + t.Namespace + "/" + t.Name + ":" + t.Version
	}

	if t.Host != "" {
		return t.Host + "/" + t.Name + ":" + t.Version
	}

	return t.Name + ":" + t.Version
}

func (t *Tag) NamespacedName() string {
	if t.Namespace != "" {
		return t.Namespace + "/" + t.Name
	}

	return t.Name
}
