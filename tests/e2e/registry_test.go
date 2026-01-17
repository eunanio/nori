//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RegistryContainer represents a running Docker registry container
type RegistryContainer struct {
	Container testcontainers.Container
	Host      string
	Port      string
	URL       string
}

// setupRegistry starts a Docker registry container using testcontainers
// Returns the registry info and a cleanup function
func setupRegistry(t *testing.T) (*RegistryContainer, func()) {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "registry:2",
		ExposedPorts: []string{"5000/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5000/tcp"),
			wait.ForHTTP("/v2/").WithPort("5000/tcp").WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}),
		).WithDeadline(60 * time.Second),
		Env: map[string]string{
			"REGISTRY_STORAGE_DELETE_ENABLED": "true",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start registry container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get container host: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5000")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get mapped port: %v", err)
	}

	registryURL := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	t.Logf("Registry started at %s", registryURL)

	reg := &RegistryContainer{
		Container: container,
		Host:      host,
		Port:      mappedPort.Port(),
		URL:       registryURL,
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("warning: failed to terminate registry container: %v", err)
		}
	}

	return reg, cleanup
}

// waitForRegistry waits for the registry to be ready and responding
func waitForRegistry(t *testing.T, registryURL string, timeout time.Duration) error {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://%s/v2/", registryURL))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("registry at %s not ready after %v", registryURL, timeout)
}

// registryReference builds an OCI reference for the test registry
func registryReference(registryURL, repo, tag string) string {
	return fmt.Sprintf("%s/%s:%s", registryURL, repo, tag)
}

