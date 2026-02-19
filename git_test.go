package amadeus

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// gitRepo is an isolated git repository backed by a testcontainer.
// Write operations (init, commit) run inside the container, completely
// bypassing host git config (GPG signing, hooks, credential helpers).
// Read operations use the host GitClient via a bind-mounted directory.
type gitRepo struct {
	container testcontainers.Container
	ctx       context.Context
	dir       string // host-side path to the repo (bind mount target)
}

func newGitRepo(t *testing.T) *gitRepo {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()

	req := testcontainers.ContainerRequest{
		Image: "alpine:3",
		Cmd:   []string{"sh", "-c", "apk add --no-cache git >/dev/null 2>&1 && echo READY && sleep infinity"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(dir, "/repo"),
		),
		WaitingFor: wait.ForLog("READY").WithStartupTimeout(30 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start git container: %v", err)
	}

	t.Cleanup(func() { c.Terminate(ctx) })

	return &gitRepo{container: c, ctx: ctx, dir: dir}
}

// git runs a git command inside the container against /repo.
func (g *gitRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := append([]string{"git", "-C", "/repo"}, args...)
	code, reader, err := g.container.Exec(g.ctx, cmd)
	if err != nil {
		t.Fatalf("exec git %v: %v", args, err)
	}
	var buf bytes.Buffer
	io.Copy(&buf, reader)
	if code != 0 {
		t.Fatalf("git %v: exit %d: %s", args, code, buf.String())
	}
	return buf.String()
}

// shell runs a shell command inside the container.
func (g *gitRepo) shell(t *testing.T, script string) {
	t.Helper()
	code, reader, err := g.container.Exec(g.ctx, []string{"sh", "-c", script})
	if err != nil {
		t.Fatalf("exec sh: %v", err)
	}
	if code != 0 {
		var buf bytes.Buffer
		io.Copy(&buf, reader)
		t.Fatalf("sh -c %q: exit %d: %s", script, code, buf.String())
	}
}

func setupTestRepo(t *testing.T) *gitRepo {
	t.Helper()
	repo := newGitRepo(t)
	repo.git(t, "init")
	repo.git(t, "config", "user.email", "test@test.com")
	repo.git(t, "config", "user.name", "Test")
	repo.git(t, "commit", "--allow-empty", "-m", "initial commit")
	repo.git(t, "commit", "--allow-empty", "-m", "Merge pull request #10 from feature/auth")
	repo.git(t, "commit", "--allow-empty", "-m", "Merge pull request #11 from feature/cart")
	return repo
}

func TestGetCurrentCommit(t *testing.T) {
	repo := setupTestRepo(t)
	git := NewGitClient(repo.dir)
	hash, err := git.CurrentCommit()
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) < 7 {
		t.Errorf("expected commit hash, got %q", hash)
	}
}

func TestGetMergedPRsSince(t *testing.T) {
	repo := setupTestRepo(t)
	git := NewGitClient(repo.dir)
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = repo.dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	initialCommit := string(out[:len(out)-1])
	prs, err := git.MergedPRsSince(initialCommit)
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Errorf("expected 2 PRs, got %d: %v", len(prs), prs)
	}
}

func TestGetDiffSince(t *testing.T) {
	repo := setupTestRepo(t)
	// Create file and commit inside the container (isolated from host hooks/GPG)
	repo.shell(t, "echo 'package main' > /repo/hello.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "add file")

	git := NewGitClient(repo.dir)
	hash, _ := git.CurrentCommit()
	diff, err := git.DiffSince(hash + "~1")
	if err != nil {
		t.Fatal(err)
	}
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}
