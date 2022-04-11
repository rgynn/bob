package bob

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	docker_types "github.com/docker/docker/api/types"
	billy "github.com/go-git/go-billy/v5"
	memfs "github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	memory "github.com/go-git/go-git/v5/storage/memory"
	moby "github.com/moby/moby/client"
)

type Builder struct {
	Docker         *moby.Client
	Writer         io.Writer
	DockerUsername string
	DockerPassword string
	DockerRepo     string
	Timeout        time.Duration
}

type BuilderOptions struct {
	Writer         io.Writer
	DockerUsername string
	DockerPassword string
	DockerHost     string
	DockerVersion  string
	DockerRepo     string
	Timeout        time.Duration
}

func NewBuilder(opts *BuilderOptions) (*Builder, error) {
	docker_client, err := moby.NewClientWithOpts(moby.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Builder{
		Docker:         docker_client,
		DockerUsername: opts.DockerUsername,
		DockerPassword: opts.DockerPassword,
		Writer:         opts.Writer,
		Timeout:        opts.Timeout,
		DockerRepo:     opts.DockerRepo,
	}, nil
}

func (b *Builder) getTarFilename(git_repo string) string {
	repos := strings.Split(git_repo, "/")
	return fmt.Sprintf("%s.tar.gz", repos[len(repos)-1])
}

func (b *Builder) Run(git_repo, git_commit, docker_image string, tags ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), b.Timeout)
	defer cancel()

	fs, err := b.Clone(ctx, git_repo, git_commit)
	if err != nil {
		return err
	}

	tarfilename := b.getTarFilename(git_repo)

	if err := b.Tar(ctx, tarfilename, fs); err != nil {
		return err
	}

	for i, tag := range tags {
		tags[i] = fmt.Sprintf("%s:%s", docker_image, tag)
	}

	if err := b.BuildImage(ctx, fs, tarfilename, docker_image, tags...); err != nil {
		return err
	}

	for _, tag := range tags {
		if err := b.Push(ctx, tag); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) Clone(ctx context.Context, repo, commit string) (billy.Filesystem, error) {
	storer := memory.NewStorage()
	fs := memfs.New()
	url := fmt.Sprintf("https://%s", repo)

	repository, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:      url,
		Progress: b.Writer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	tree, err := repository.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := tree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	}); err != nil {
		return nil, fmt.Errorf(
			"failed to checkout repo: %s, commit: %s, error: %w",
			repo,
			commit,
			err,
		)
	}

	return fs, nil
}

func (b *Builder) Tar(ctx context.Context, tarfilename string, fs billy.Filesystem) error {
	file, err := fs.Create(tarfilename)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := b.addDirectoryToArchive(".", tarfilename, fs, tw); err != nil {
		return err
	}

	if err := tw.Flush(); err != nil {
		return err
	}

	return nil
}

func (b *Builder) addDirectoryToArchive(path, tarfilename string, fs billy.Filesystem, tw *tar.Writer) error {
	files, err := fs.ReadDir(path)
	if err != nil {
		return err
	}

	for _, fd := range files {
		path := fmt.Sprintf("%s/%s", path, fd.Name())

		if strings.Contains(path, tarfilename) ||
			strings.Contains(path, ".git") ||
			strings.Contains(path, ".github") {
			continue
		}

		if fd.IsDir() {
			if err := b.addDirectoryToArchive(path, tarfilename, fs, tw); err != nil {
				return err
			}
			continue
		}

		if err := b.addFileToArchive(path, fs, tw); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) addFileToArchive(path string, fs billy.Filesystem, tw *tar.Writer) error {
	file, err := fs.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := fs.Stat(path)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = path

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

func (b *Builder) DumpArchive(tarfilename string, fs billy.Filesystem) error {
	src, err := fs.Open(tarfilename)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create("tmp/" + src.Name())
	if err != nil {
		return err
	}

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	if err := dst.Close(); err != nil {
		return err
	}

	return nil
}

func (b *Builder) BuildImage(ctx context.Context, fs billy.Filesystem, tarfilename, docker_image string, tags ...string) error {
	file, err := fs.Open(tarfilename)
	if err != nil {
		return fmt.Errorf("failed to open tar file in mem fs: %w", err)
	}
	defer file.Close()

	resp, err := b.Docker.ImageBuild(
		ctx,
		file,
		docker_types.ImageBuildOptions{
			Tags:        tags,
			NoCache:     false,
			Remove:      true,
			ForceRemove: true,
			PullParent:  true,
			Dockerfile:  "Dockerfile",
		})
	if err != nil {
		return fmt.Errorf("failed to build image in docker daemon: %w", err)
	}
	defer resp.Body.Close()

	type msg struct {
		Text string `json:"stream"`
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		var msg msg
		if err := json.NewDecoder(bytes.NewReader(scanner.Bytes())).Decode(&msg); err != nil {
			return err
		}
		if b.Writer != nil {
			if _, err := b.Writer.Write([]byte(msg.Text)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Builder) Push(ctx context.Context, tag string) error {
	authConfig, err := b.getAuthConfig()
	if err != nil {
		return err
	}

	resp, err := b.Docker.ImagePush(
		ctx,
		tag,
		docker_types.ImagePushOptions{
			RegistryAuth: authConfig,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer resp.Close()

	type msg struct {
		Text  string  `json:"status"`
		Error *string `json:"error"`
	}

	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		var msg msg
		if err := json.NewDecoder(bytes.NewReader(scanner.Bytes())).Decode(&msg); err != nil {
			return err
		}
		if msg.Error != nil {
			return errors.New(*msg.Error)
		}
		if b.Writer != nil {
			if _, err := b.Writer.Write([]byte(msg.Text + "\n")); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Builder) getAuthConfig() (string, error) {
	cfg := docker_types.AuthConfig{
		Username:      b.DockerUsername,
		Password:      b.DockerPassword,
		ServerAddress: fmt.Sprintf("%s/v1/", b.DockerRepo),
	}
	authConfigBytes, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(authConfigBytes), nil
}
