//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// sharedImage holds the Docker image name built once by TestMain.
// Docker-based tests use this instead of rebuilding per-test.
var sharedImage string

func TestMain(m *testing.M) {
	// Build the Docker image once before all tests.
	ctx := context.Background()

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       repoRoot(),
				Dockerfile:    "tests/e2e/Dockerfile.e2e",
				PrintBuildLog: true,
			},
			Cmd: []string{"sleep", "infinity"},
			WaitingFor: wait.ForExec([]string{"amadeus", "--version"}).
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	}

	// Build and start a throwaway container to cache the image.
	c, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: failed to build shared image: %v\n", err)
		os.Exit(1)
	}

	inspect, err := c.Inspect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: failed to inspect container: %v\n", err)
		c.Terminate(ctx)
		os.Exit(1)
	}
	tag := "amadeus-e2e:latest"
	tagCmd := exec.CommandContext(ctx, "docker", "tag", inspect.Image, tag)
	if err := tagCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: failed to tag image: %v\n", err)
		c.Terminate(ctx)
		os.Exit(1)
	}
	sharedImage = tag

	// Terminate the bootstrap container.
	c.Terminate(ctx)

	os.Exit(m.Run())
}

// repoRoot returns the amadeus repository root relative to the test working directory.
func repoRoot() string {
	return "../.."
}
