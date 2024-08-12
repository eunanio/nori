package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/spec"
)

type PushBlobOptions struct {
	Digest   spec.Digest
	File     []byte
	Name     string
	Insecure bool
	Tag      *spec.Tag
}

type PullBlobOptions struct {
	Digest spec.Digest
	Name   string
	Tag    *spec.Tag
}

type PushManifestOptions struct {
	Manifest *spec.Manifest
	Tag      *spec.Tag
	Insecure bool
}

func (r *Registry) PullManifest(tag *spec.Tag) (manifest *spec.Manifest, err error) {
	manifest, err = futils.GetTaggedManifest(tag)
	if err != nil {
		return nil, fmt.Errorf("error getting manifest sha: %s", err.Error())
	}
	if manifest != nil {
		fmt.Println("Using cached manifest")
		return manifest, nil
	}

	var endpoint string
	if tag.Namespace != "" {
		endpoint = fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s", r.Url, tag.Namespace, tag.Name, tag.Version)
	} else {
		endpoint = fmt.Sprintf("https://%s/v2/%s/manifests/%s", r.Url, tag.Name, tag.Version)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %s", err.Error())
	}

	req.Header.Add("Accept", spec.MEDIA_TYPE_MANIFEST)
	if r.Auth != "" {
		req.Header.Add("Authorization", r.Auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %s", err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("unauthorized, please use nori login to authenticate")
		}

		return nil, fmt.Errorf("cannot to pull manifest: %s", resp.Status)
	}
	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest: %s", err.Error())
	}

	err = json.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling manifest: %s", err.Error())
	}

	return manifest, nil
}

func (r *Registry) PushManifest(opts PushManifestOptions) error {
	jsonBytes, err := json.Marshal(opts.Manifest)
	if err != nil {
		return fmt.Errorf("error marshalling manifest: %s", err.Error())
	}

	var protocol string
	if opts.Insecure {
		protocol = "http"
	} else {
		protocol = "https"
	}
	var endpoint string
	if opts.Tag.Namespace != "" {
		endpoint = fmt.Sprintf("%s://%s/v2/%s/%s/manifests/%s", protocol, r.Url, opts.Tag.Namespace, opts.Tag.Name, opts.Tag.Version)
	} else {
		endpoint = fmt.Sprintf("%s://%s/v2/%s/manifests/%s", protocol, r.Url, opts.Tag.Name, opts.Tag.Version)
	}

	req, err := http.NewRequest("HEAD", endpoint, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %s", err.Error())
	}

	if r.Auth != "" {
		req.Header.Add("Authorization", r.Auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %s", err.Error())
	}

	if resp.StatusCode != 200 {
		uploadReq, err := http.NewRequest("PUT", endpoint, bytes.NewReader(jsonBytes))
		if err != nil {
			return fmt.Errorf("error creating request: %s", err.Error())
		}

		uploadReq.Header.Add("Content-Type", spec.MEDIA_TYPE_MANIFEST)

		if r.Auth != "" {
			uploadReq.Header.Add("Authorization", r.Auth)
		}

		resp, err = client.Do(uploadReq)
		if err != nil {
			return fmt.Errorf("error sending request: %s", err.Error())
		}

		if resp.StatusCode != 201 {
			if resp.StatusCode == http.StatusUnauthorized {
				return fmt.Errorf("unauthorized, please use nori login to authenticate")
			}
			return fmt.Errorf("failed to push manifest: %s", resp.Status)
		}
	}

	return nil
}

func (r *Registry) PullBlob(opts PullBlobOptions) (data []byte, err error) {
	var endpoint string
	if opts.Tag.Namespace != "" {
		endpoint = fmt.Sprintf("https://%s/v2/%s/%s/blobs/%s", r.Url, opts.Tag.Namespace, opts.Tag.Name, opts.Digest.Digest)
	} else {
		endpoint = fmt.Sprintf("https://%s/v2/%s/blobs/%s", r.Url, opts.Tag.Name, opts.Digest.Digest)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %s", err.Error())
	}

	if r.Auth != "" {
		req.Header.Add("Authorization", r.Auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %s", err.Error())
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("unauthorized, please use nori login to authenticate")
		}
		return nil, fmt.Errorf("failed to pull blob: %s", resp.Status)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading blob: %s", err.Error())
	}

	return data, nil
}

func (r *Registry) PushBlob(opts PushBlobOptions) error {
	var protocol string
	if opts.Insecure {
		protocol = "http"
	} else {
		protocol = "https"
	}

	var endpoint string
	if opts.Tag.Namespace != "" {
		endpoint = fmt.Sprintf("%s://%s/v2/%s/%s/blobs/uploads/", protocol, r.Url, opts.Tag.Namespace, opts.Tag.Name)
	} else {
		endpoint = fmt.Sprintf("%s://%s/v2/%s/blobs/uploads/", protocol, r.Url, opts.Tag.Name)
	}

	// Initiate the upload session
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return err
	}

	if r.Auth != "" {
		req.Header.Add("Authorization", r.Auth)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 202 {
		fmt.Println(resp.StatusCode)
		return fmt.Errorf("failed to push blob: %s", resp.Status)
	}

	// Upload the blob
	location := resp.Header.Get("Location")
	req, err = http.NewRequest("PUT", location, bytes.NewReader(opts.File))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	query := req.URL.Query()
	query.Add("digest", opts.Digest.Digest)
	req.URL.RawQuery = query.Encode()

	if r.Auth != "" {
		req.Header.Add("Authorization", r.Auth)
	}

	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("unauthorized, please use nori login to authenticate")
		}
		return fmt.Errorf("failed to push blob: %s", resp.Status)
	}

	fmt.Printf("%s: pushed\n", opts.Digest.Digest[:24])
	return nil
}
