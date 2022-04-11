package main

import (
	"bob"
	"log"
	"os"
	"time"
)

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)
	builder, err := bob.NewBuilder(&bob.BuilderOptions{
		Logger:        logger,
		DockerHost:    "unix:///var/run/docker.sock",
		DockerVersion: "1.41",
		Organisation:  "rgynn",
		DockerRepo:    "hub.docker.com",
		Timeout:       time.Minute * 5,
	})
	if err != nil {
		logger.Fatal(err)
	}
	commit := "626689284788951e300dd5847cef859d711d2266"
	tags := []string{"latest", commit}
	if err := builder.Run("klottr", commit, "klottr", tags...); err != nil {
		logger.Fatal(err)
	}
}
