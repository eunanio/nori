package futils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/eunanhardy/nori/internal/spec"
)



func ParseImageTag(tag string) (*spec.Tag, error) {
	var host, name, version string

	if tag == "" {
		return nil, fmt.Errorf("invalid tag: tag cannot be empty")
	}

	nameAndVersion := strings.Split(tag, ":")
	
	if len(nameAndVersion) > 2 {
		version = nameAndVersion[2]
		name = nameAndVersion[0] + ":" + nameAndVersion[1]
	} else {
		name = nameAndVersion[0]
		if len(nameAndVersion) > 1 {
			version = nameAndVersion[1]
		} else {
			version = "latest"
		}
	}

	nameParts := strings.SplitN(name, "/", 2)

	if len(nameParts) > 1 && (strings.Contains(nameParts[0], ".") || strings.Contains(nameParts[0], ":")) {
		host = nameParts[0]
		name = nameParts[1]
	}

	if name == "" {
		return nil, fmt.Errorf("invalid tag: tag must include a name")
	}

	return &spec.Tag{
		Host:    host,
		Name:    name,
		Version: version,
	},nil
}

func ParseTagV2(tag string) (*spec.Tag, error) {
	pattern := `^(?:(?P<host>[a-zA-Z0-9.-]+(?::[0-9]+)?)\/)?(?:(?P<namespace>[a-zA-Z0-9-._]+)\/)?(?P<name>[a-zA-Z0-9-._]+)(?::(?P<tag>[a-zA-Z0-9-._]+))?$`
	re := regexp.MustCompile(pattern)

	// Match the input image URL against the pattern.
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return nil, fmt.Errorf("invalid Docker image URL")
	}

	// Extract the captured groups into a map.
	groupNames := re.SubexpNames()
	result := make(map[string]string)
	for i, name := range groupNames {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}
	image := spec.Tag{
		Host:      result["host"],
		Namespace: result["namespace"],
		Name:      result["name"],
		Version:   result["tag"],
	}

	if image.Version == "" {
		image.Version = "latest"
	}

	return &image, nil
}
