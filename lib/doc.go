// Package nori provides a high-level Go API for OCI-based Terraform/OpenTofu
// module management.
//
// It wraps the lower-level packages under github.com/eunanio/nori/pkg into a
// single Client that can package, push, pull, inspect, deploy, and manage
// releases of infrastructure modules stored in OCI registries.
//
// # Quick start
//
//	import nori "github.com/eunanio/nori/lib"
//
//	client, err := nori.NewClient(
//	    nori.WithConfigPath("~/.nori/config.yaml"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Pull a module
//	result, err := client.Pull(ctx, "ghcr.io/myorg/s3-bucket:v1.0.0", nori.PullOptions{})
//
//	// Create a release
//	rel, err := client.CreateRelease(ctx, "my-bucket", "ghcr.io/myorg/s3-bucket:v1.0.0",
//	    nori.CreateReleaseOptions{
//	        ValuesFile:  "values.yaml",
//	        AutoApprove: true,
//	    },
//	)
package nori
