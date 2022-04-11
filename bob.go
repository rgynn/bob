package bob

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
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
	Docker       *moby.Client
	Organisation string
	Timeout      time.Duration
}

type BuilderOptions struct {
	DockerHost    string
	DockerVersion string
	Organisation  string
	Timeout       time.Duration
}

func NewBuilder(opts *BuilderOptions) (*Builder, error) {
	docker_client, err := moby.NewClientWithOpts(moby.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Builder{
		Docker:       docker_client,
		Timeout:      opts.Timeout,
		Organisation: opts.Organisation,
	}, nil
}

func (b *Builder) Clone(ctx context.Context, repo, commit string) (billy.Filesystem, error) {
	var buffer []byte
	progress := bytes.NewBuffer(buffer)

	storer := memory.NewStorage()
	fs := memfs.New()
	url := fmt.Sprintf("https://github.com/%s/%s", b.Organisation, repo)

	repository, err := git.Clone(storer, fs, &git.CloneOptions{
		URL:      url,
		Progress: progress,
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

	scanner := bufio.NewScanner(progress)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			break
		}
		log.Printf("GIT\t%s\n", scanner.Text())
	}

	return fs, nil
}

func (b *Builder) Tar(ctx context.Context, repo string, fs billy.Filesystem) (billy.File, error) {

	tarfilename := fmt.Sprintf("%s.tar.gz", repo)

	file, err := fs.Create(tarfilename)
	if err != nil {
		return nil, err
	}

	buffer := bufio.NewWriter(file)

	gw := gzip.NewWriter(buffer)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := b.addDirectoryToArchive(".", tarfilename, fs, tw); err != nil {
		return nil, err
	}

	return file, nil
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

		fmt.Printf("Adding to archive: %s\n", path)
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

	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) BuildImage(ctx context.Context, file billy.File, image string, tags ...string) error {
	resp, err := b.Docker.ImageBuild(
		ctx,
		file,
		docker_types.ImageBuildOptions{
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
		log.Printf("BUILD\t%s\n", scanner.Text())
	}

	return nil
}

func (b *Builder) Push(ctx context.Context, image string) error {
	resp, err := b.Docker.ImagePush(
		ctx,
		image,
		docker_types.ImagePushOptions{},
	)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			break
		}
		log.Printf("PUSH\t%s\n", scanner.Text())
	}

	return nil
}

func (b *Builder) Run(repo, commit, image string, tags ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), b.Timeout)
	defer cancel()

	fs, err := b.Clone(ctx, repo, commit)
	if err != nil {
		return err
	}

	file, err := b.Tar(ctx, repo, fs)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := b.BuildImage(ctx, file, image, tags...); err != nil {
		return err
	}

	for _, tag := range tags {
		if err := b.Push(ctx, fmt.Sprintf("%s:%s", image, tag)); err != nil {
			return err
		}
	}

	return nil
}
