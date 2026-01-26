package signing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// signatureLayer implements v1.Layer for signature content.
type signatureLayer struct {
	payload   []byte
	signature []byte
}

func (l *signatureLayer) Digest() (v1.Hash, error) {
	r, err := l.Compressed()
	if err != nil {
		return v1.Hash{}, err
	}
	defer r.Close()
	h, _, err := v1.SHA256(r)
	return h, err
}

func (l *signatureLayer) DiffID() (v1.Hash, error) {
	return l.Digest()
}

func (l *signatureLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.payload)), nil
}

func (l *signatureLayer) Uncompressed() (io.ReadCloser, error) {
	return l.Compressed()
}

func (l *signatureLayer) Size() (int64, error) {
	return int64(len(l.payload)), nil
}

func (l *signatureLayer) MediaType() (types.MediaType, error) {
	return types.MediaType(SignatureMediaType), nil
}

// signatureImage implements v1.Image for a cosign signature.
type signatureImage struct {
	layer *signatureLayer
}

func (i *signatureImage) Layers() ([]v1.Layer, error) {
	return []v1.Layer{i.layer}, nil
}

func (i *signatureImage) MediaType() (types.MediaType, error) {
	return types.OCIManifestSchema1, nil
}

func (i *signatureImage) Size() (int64, error) {
	return i.layer.Size()
}

func (i *signatureImage) ConfigName() (v1.Hash, error) {
	return i.configDigest()
}

func (i *signatureImage) ConfigFile() (*v1.ConfigFile, error) {
	return &v1.ConfigFile{
		Created: v1.Time{Time: time.Now()},
	}, nil
}

func (i *signatureImage) RawConfigFile() ([]byte, error) {
	return []byte("{}"), nil
}

func (i *signatureImage) Digest() (v1.Hash, error) {
	raw, err := i.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	return h, err
}

func (i *signatureImage) Manifest() (*v1.Manifest, error) {
	layerDigest, err := i.layer.Digest()
	if err != nil {
		return nil, err
	}

	layerSize, err := i.layer.Size()
	if err != nil {
		return nil, err
	}

	configDigest, err := i.configDigest()
	if err != nil {
		return nil, err
	}

	return &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.OCIConfigJSON,
			Size:      2, // "{}"
			Digest:    configDigest,
		},
		Layers: []v1.Descriptor{
			{
				MediaType: types.MediaType(SignatureMediaType),
				Size:      layerSize,
				Digest:    layerDigest,
				Annotations: map[string]string{
					AnnotationSignature: base64.StdEncoding.EncodeToString(i.layer.signature),
				},
			},
		},
	}, nil
}

func (i *signatureImage) RawManifest() ([]byte, error) {
	m, err := i.Manifest()
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (i *signatureImage) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	layerDigest, err := i.layer.Digest()
	if err != nil {
		return nil, err
	}
	if hash == layerDigest {
		return i.layer, nil
	}
	return nil, nil
}

func (i *signatureImage) LayerByDiffID(hash v1.Hash) (v1.Layer, error) {
	return i.LayerByDigest(hash)
}

func (i *signatureImage) configDigest() (v1.Hash, error) {
	h, _, err := v1.SHA256(bytes.NewReader([]byte("{}")))
	return h, err
}

