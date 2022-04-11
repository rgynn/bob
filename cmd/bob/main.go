package main

import (
	"bob"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	git_repo        string
	git_commit      string
	docker_image    string
	docker_username string
	docker_password string
	tags_flag       string
	timeout         time.Duration
)

func init() {
	flag.StringVar(&git_repo, "git-repo", "", "git repository to checkout")
	flag.StringVar(&git_commit, "commit", "", "git commit to checkout from repository and tag docker image with")
	flag.StringVar(&docker_image, "docker-image", "", "docker image to push")
	flag.StringVar(&docker_username, "u", "00000000-0000-0000-0000-000000000000", "docker repository user to push with")
	flag.StringVar(&docker_password, "p", "", "docker repository password to push with")
	flag.StringVar(&tags_flag, "tags", "", "additional tags to push docker image with")
	flag.DurationVar(&timeout, "timeout", time.Minute*5, "timeout for job")
	flag.Parse()
}

func main() {
	builder, err := bob.NewBuilder(&bob.BuilderOptions{
		Writer:         os.Stdout,
		DockerUsername: docker_username,
		DockerPassword: docker_password,
		Timeout:        timeout,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := builder.Run(git_repo, git_commit, docker_image, getTags()...); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getTags() (result []string) {
	default_tags := []string{"latest", git_commit}
	if tags_flag != "" {
		tags_flag = strings.ReplaceAll(tags_flag, " ", "")
		result = append(result, strings.Split(tags_flag, ",")...)
	}
	return append(result, default_tags...)
}
