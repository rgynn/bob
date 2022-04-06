package bob

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	git "github.com/go-git/go-git/v5"
	github "github.com/google/go-github/github"
	archiver "github.com/mholt/archiver/v4"
)

type Builder struct {
	Docker       *docker.Client
	Github       *github.Client
	Reader       io.Reader
	Writer       io.Writer
	Organisation string
	Timeout      time.Duration
}

type BuilderOptions struct {
	Reader        io.Reader
	Writer        io.Writer
	DockerHost    string
	DockerVersion string
	Organisation  string
	Timeout       time.Duration
}

func NewBuilder(opts *BuilderOptions) (*Builder, error) {
	http_client := &http.Client{}
	headers := map[string]string{}

	docker_client, err := docker.NewClient(
		opts.DockerHost,
		opts.DockerVersion,
		http_client,
		headers,
	)
	if err != nil {
		return nil, err
	}

	github_client := github.NewClient(http_client)

	return &Builder{
		Docker:       docker_client,
		Github:       github_client,
		Timeout:      opts.Timeout,
		Organisation: opts.Organisation,
		Writer:       opts.Writer,
		Reader:       opts.Reader,
	}, nil
}

func (b *Builder) Clone(ctx context.Context, src, dest string) error {
	if _, err := git.PlainClone(dest, false, &git.CloneOptions{
		URL:      fmt.Sprintf("https://github.com/%s/%s", b.Organisation, src),
		Progress: b.Writer,
	}); err != nil {
		return err
	}
	return nil
}

func (b *Builder) Tar(ctx context.Context, target string) (io.Reader, error) {
	files, err := archiver.FilesFromDisk(nil, map[string]string{
		target: target,
	})
	if err != nil {
		return nil, err
	}

	var buffer []byte
	out := bytes.NewBuffer(buffer)
	format := archiver.CompressedArchive{
		Compression: archiver.Gz{},
		Archival:    archiver.Tar{},
	}

	if err := format.Archive(ctx, out, files); err != nil {
		return nil, err
	}

	return out, nil
}

func (b *Builder) BuildImage(ctx context.Context, file io.Reader, image string, tags ...string) error {
	resp, err := b.Docker.ImageBuild(
		ctx,
		file,
		types.ImageBuildOptions{
			Tags:        tags,
			NoCache:     true,
			Remove:      true,
			ForceRemove: true,
			PullParent:  true,
			Dockerfile:  "Dockerfile",
		})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			break
		}
		log.Println(scanner.Text())
	}

	return nil
}

func (b *Builder) Push(ctx context.Context, image string) error {
	resp, err := b.Docker.ImagePush(
		ctx,
		image,
		types.ImagePushOptions{},
	)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			break
		}
		log.Println(scanner.Text())
	}

	return nil
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

	for _, tag := range tags {
		if err := b.Push(ctx, fmt.Sprintf("%s:%s", image, tag)); err != nil {
			return err
		}
	}

	return nil
}
