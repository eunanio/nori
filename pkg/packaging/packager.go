// Package packaging provides module packaging functionality.
package packaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eunanio/nori/internal/util"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
)

// Packager handles module packaging operations.
type Packager struct {
	client *oci.Client
	logger *slog.Logger
}

// PackageOptions contains options for packaging a module.
type PackageOptions struct {
	// Description is the module description.
	Description string

	// ConfigPath is the path to a Terraform configuration for metadata extraction.
	ConfigPath string

	// Annotations are custom OCI annotations to add.
	Annotations map[string]string

	// Insecure allows HTTP connections.
	Insecure bool
}

// ModuleMetadata contains extracted module metadata.
type ModuleMetadata struct {
	Name        string            `json:"name,omitempty"`
	Version     string            `json:"version,omitempty"`
	Description string            `json:"description,omitempty"`
	Source      string            `json:"source,omitempty"`
	Variables   []VariableInfo    `json:"variables,omitempty"`
	Outputs     []OutputInfo      `json:"outputs,omitempty"`
	Providers   []ProviderInfo    `json:"providers,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VariableInfo contains information about a Terraform variable.
type VariableInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Required    bool        `json:"required"`
}

// OutputInfo contains information about a Terraform output.
type OutputInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

// ProviderInfo contains information about a required provider.
type ProviderInfo struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}

// PackageResult contains the result of a packaging operation.
type PackageResult struct {
	Reference   string
	Digest      string
	Size        int64
	Annotations map[string]string
}

// PrepareResult contains the result of preparing a package (without pushing).
type PrepareResult struct {
	Content       []byte
	Annotations   map[string]string
	Size          int64
	ReadmeContent []byte
}

// NewPackager creates a new Packager.
func NewPackager(client *oci.Client, logger *slog.Logger) *Packager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Packager{
		client: client,
		logger: logger,
	}
}

// PreparePackage prepares a module archive for packaging without pushing to a registry.
// It validates, converts (if needed), and builds annotations.
// OpenTofu requires modules to be in zip format with archive/zip media type.
func (p *Packager) PreparePackage(ctx context.Context, archivePath string, opts PackageOptions) (*PrepareResult, error) {
	p.logger.Info("preparing package", "archive", archivePath)

	// Validate archive
	if err := util.ValidateArchive(archivePath); err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	// Read archive content
	content, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive: %w", err)
	}

	// OpenTofu requires zip format - convert tar.gz to zip if needed
	archiveType := util.DetectArchiveType(archivePath)
	if archiveType == util.ArchiveTypeTarGz {
		p.logger.Debug("converting tar.gz to zip for OpenTofu compatibility")
		content, err = util.ConvertTarGzToZip(archivePath)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tar.gz to zip: %w", err)
		}
	}
	// Zip files are used as-is (OpenTofu native format)

	// Build annotations
	annotations := p.buildAnnotations(archivePath, opts)

	// Extract metadata if config path is provided
	if opts.ConfigPath != "" {
		metadata, err := p.extractMetadata(opts.ConfigPath)
		if err != nil {
			p.logger.Warn("failed to extract metadata", "error", err)
		} else {
			p.addMetadataAnnotations(annotations, metadata)
		}
	}

	// Extract README from archive
	var readmeContent []byte
	readmeContent, err = p.extractReadme(archivePath, content, archiveType)
	if err != nil {
		p.logger.Warn("failed to extract README", "error", err)
	}
	if readmeContent != nil {
		p.logger.Debug("README.md detected in archive", "size", len(readmeContent))
		annotations[oci.AnnotationReadme] = "true"
	}

	return &PrepareResult{
		Content:       content,
		Annotations:   annotations,
		Size:          int64(len(content)),
		ReadmeContent: readmeContent,
	}, nil
}

// Package packages a module archive and pushes it to a registry.
// OpenTofu requires modules to be in zip format with archive/zip media type.
func (p *Packager) Package(ctx context.Context, reference, archivePath string, opts PackageOptions) (*PackageResult, error) {
	p.logger.Info("packaging module", "reference", reference, "archive", archivePath)

	// Parse reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	// Prepare the package
	prepared, err := p.PreparePackage(ctx, archivePath, opts)
	if err != nil {
		return nil, err
	}

	// Push artifact with optional README layer
	artifact, err := p.client.PushArtifactWithReadme(ctx, ref, prepared.Content, prepared.ReadmeContent, prepared.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to push artifact: %w", err)
	}

	return &PackageResult{
		Reference:   reference,
		Digest:      artifact.Digest,
		Size:        prepared.Size,
		Annotations: artifact.Annotations,
	}, nil
}

// buildAnnotations builds the OCI annotations for the artifact.
func (p *Packager) buildAnnotations(archivePath string, opts PackageOptions) map[string]string {
	annotations := make(map[string]string)

	// Set title from filename
	annotations[oci.AnnotationTitle] = filepath.Base(archivePath)

	// Set creation time
	annotations[oci.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)

	// Set description if provided
	if opts.Description != "" {
		annotations[oci.AnnotationDescription] = opts.Description
	}

	// Add custom annotations
	for k, v := range opts.Annotations {
		annotations[k] = v
	}

	return annotations
}

// addMetadataAnnotations adds metadata to annotations.
func (p *Packager) addMetadataAnnotations(annotations map[string]string, metadata *ModuleMetadata) {
	if metadata.Description != "" && annotations[oci.AnnotationDescription] == "" {
		annotations[oci.AnnotationDescription] = metadata.Description
	}
	if metadata.Version != "" {
		annotations[oci.AnnotationVersion] = metadata.Version
	}
	if metadata.Source != "" {
		annotations[oci.AnnotationSource] = metadata.Source
	}

	// Add metadata as JSON annotation
	metadataJSON, err := json.Marshal(metadata)
	if err == nil {
		annotations["io.nori.module.metadata"] = string(metadataJSON)
	}
}

// extractMetadata extracts metadata from Terraform configuration files.
func (p *Packager) extractMetadata(configPath string) (*ModuleMetadata, error) {
	metadata := &ModuleMetadata{}

	// Check if configPath is a file or directory
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat config path: %w", err)
	}

	var configDir string
	if info.IsDir() {
		configDir = configPath
	} else {
		configDir = filepath.Dir(configPath)
	}

	// Find and parse .tf files
	files, err := filepath.Glob(filepath.Join(configDir, "*.tf"))
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform files: %w", err)
	}

	for _, file := range files {
		if err := p.parseConfigFile(file, metadata); err != nil {
			p.logger.Debug("failed to parse config file", "file", file, "error", err)
		}
	}

	return metadata, nil
}

// parseConfigFile parses a single Terraform configuration file.
// This is a simplified parser - in production, you'd want to use HCL parser.
func (p *Packager) parseConfigFile(path string, metadata *ModuleMetadata) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	var currentBlock string
	var blockName string
	var description string
	var hasDefault bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		// Detect block starts
		if strings.HasPrefix(line, "variable ") {
			currentBlock = "variable"
			blockName = extractBlockName(line)
			description = ""
			hasDefault = false
		} else if strings.HasPrefix(line, "output ") {
			currentBlock = "output"
			blockName = extractBlockName(line)
			description = ""
		} else if strings.HasPrefix(line, "terraform ") {
			currentBlock = "terraform"
		} else if strings.HasPrefix(line, "}") {
			// Block end - save collected data
			switch currentBlock {
			case "variable":
				metadata.Variables = append(metadata.Variables, VariableInfo{
					Name:        blockName,
					Description: description,
					Required:    !hasDefault,
				})
			case "output":
				metadata.Outputs = append(metadata.Outputs, OutputInfo{
					Name:        blockName,
					Description: description,
				})
			}
			currentBlock = ""
		}

		// Parse block content
		if strings.Contains(line, "description") && strings.Contains(line, "=") {
			description = extractStringValue(line)
		}
		if strings.Contains(line, "default") && strings.Contains(line, "=") {
			hasDefault = true
		}
	}

	return nil
}

// extractBlockName extracts the name from a block declaration line.
func extractBlockName(line string) string {
	// Remove block type
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}

	name := parts[1]
	// Remove quotes if present
	name = strings.Trim(name, "\"'{}")
	return name
}

// extractStringValue extracts a string value from an assignment line.
func extractStringValue(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}

	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "\"'")
	return value
}

// extractReadme extracts README.md content from an archive.
func (p *Packager) extractReadme(archivePath string, content []byte, archiveType util.ArchiveType) ([]byte, error) {
	switch archiveType {
	case util.ArchiveTypeZip:
		// For zip files, extract from the content (which may have been converted)
		return util.ExtractReadmeFromZip(content)
	case util.ArchiveTypeTarGz:
		// For tar.gz, extract from the original file before conversion
		return util.ExtractReadmeFromTarGz(archivePath)
	default:
		return nil, nil
	}
}
