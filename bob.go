package bob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	docker "github.com/docker/docker/client"
	github "github.com/google/go-github/github"
)

type Builder struct {
	Docker  *docker.Client
	Github  *github.Client
	Timeout time.Duration
}

func NewBuilder(host, version string, timeout time.Duration) (*Builder, error) {
	http_client := &http.Client{}
	headers := map[string]string{}

	docker_client, err := docker.NewClient(host, version, http_client, headers)
	if err != nil {
		return nil, err
	}

	github_client := github.NewClient(http_client)

	return &Builder{
		Docker:  docker_client,
		Github:  github_client,
		Timeout: timeout,
	}, nil
}

func (b *Builder) Clone(ctx context.Context, src, dest string) error {
	return errors.New("not implemented yet")
}

func (b *Builder) Tar(ctx context.Context, target string) (io.Reader, error) {
	return nil, errors.New("not implemented yet")
}

func (b *Builder) BuildImage(ctx context.Context, file io.Reader, image string, tags ...string) error {
	return errors.New("not implemented yet")
}

func (b *Builder) Push(ctx context.Context, image string, tags ...string) error {
	return errors.New("not implemented yet")
}

func (b *Builder) Run(repo, image string, tags ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), b.Timeout)
	defer cancel()

	tmpdir := fmt.Sprintf("./tmp/%s", repo)

	if err := b.Clone(ctx, repo, tmpdir); err != nil {
		return err
	}

	tar, err := b.Tar(ctx, tmpdir)
	if err != nil {
		return err
	}

	if err := b.BuildImage(ctx, tar, image, tags...); err != nil {
		return err
	}

	if err := b.Push(ctx, image, tags...); err != nil {
		return err
	}

	return nil
}
