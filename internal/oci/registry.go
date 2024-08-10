package oci

import (
	"fmt"
)

type Registry struct {
	Url  string
	Auth string
}

func NewRegistry(url, auth string) *Registry {
	if auth != "" {
		auth = fmt.Sprintf("Basic %s", auth)
	}

	return &Registry{
		Url:  url,
		Auth: auth,
	}
}

