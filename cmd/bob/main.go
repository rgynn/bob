package main

import (
	"bob"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var (
	git_repo        string
	docker_repo     string
	docker_username string
	docker_password string
	org             string
	commit          string
	tags_flag       string
	timeout         time.Duration
)

func init() {
	flag.StringVar(&git_repo, "git-repo", "", "git repository to checkout")
	flag.StringVar(&docker_repo, "docker-repo", "", "docker repository to push to")
	flag.StringVar(&docker_username, "u", "00000000-0000-0000-0000-000000000000", "docker repository user to push with")
	flag.StringVar(&docker_password, "p", "", "docker repository password to push with")
	flag.StringVar(&org, "org", "", "git and docker organisation to checkout")
	flag.StringVar(&commit, "commit", "", "git commit to checkout from repository and tag docker image with")
	flag.StringVar(&tags_flag, "tags", "", "additional tags to push docker image with")
	flag.DurationVar(&timeout, "timeout", time.Minute*5, "timeout for job")
	flag.Parse()
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	builder, err := bob.NewBuilder(&bob.BuilderOptions{
		Writer:         os.Stdout,
		DockerHost:     "unix:///var/run/docker.sock",
		DockerUsername: docker_username,
		DockerPassword: docker_password,
		DockerVersion:  "1.41",
		Organisation:   org,
		DockerRepo:     docker_repo,
		Timeout:        timeout,
	})
	if err != nil {
		logger.Fatal(err)
	}
	default_tags := []string{"latest", commit}

	var tags []string
	if tags_flag != "" {
		tags_flag = strings.ReplaceAll(tags_flag, " ", "")
		tags = append(tags, strings.Split(tags_flag, ",")...)
	}
	tags = append(tags, default_tags...)

	if err := builder.Run(git_repo, commit, git_repo, tags...); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
