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
		Timeout:       time.Minute * 5,
	})
	if err != nil {
		logger.Fatal(err)
	}
	if err := builder.Run("klottr", "626689284788951e300dd5847cef859d711d2266", "latest"); err != nil {
		logger.Fatal(err)
	}
}
