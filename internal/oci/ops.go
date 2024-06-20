package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/eunanhardy/nori/internal/spec"
)

type PushBlobOptions struct {
	Digest spec.Digest
	File   []byte
	Name  string
	Insecure bool
}

type PullBlobOptions struct {
	Digest spec.Digest
	Name string
}

type PushManifestOptions struct {
	Manifest spec.Manifest
	Tag      *spec.Tag
	Insecure bool
}

func (r *Registry) PullManifest(tag *spec.Tag) (manifest *spec.Manifest, err error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v2/%s/manifests/%s", r.Url, tag.Name, tag.Version), nil)
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

func (r *Registry) PushManifest(opts PushManifestOptions) (error) {
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

	endpopint := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", protocol, r.Url, opts.Tag.Name, opts.Tag.Version)

	req, err := http.NewRequest("HEAD", endpopint, nil)
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
		uploadReq, err := http.NewRequest("PUT", endpopint, bytes.NewReader(jsonBytes))
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
			fmt.Println(resp.Body)
			return fmt.Errorf("failed to push manifest: %s", resp.Status)
		}
	}

	return nil
}

func (r *Registry) PullBlob(opts PullBlobOptions) (data []byte, err error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v2/%s/blobs/%s", r.Url, opts.Name, opts.Digest.Digest), nil)
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

	// Initiate the upload session
	req, err := http.NewRequest("POST", fmt.Sprintf("%s://%s/v2/%s/blobs/uploads/", protocol,r.Url, opts.Name), nil)
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

	resp, err = client.Do(req); if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("failed to push blob: %s", resp.Status)
	}

	fmt.Printf("%s: pushed\n", opts.Digest.Digest[7:])
	return nil
}