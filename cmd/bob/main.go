package main

import (
	"log"
	"time"
	bob "tooling-bob"
)

func main() {
	builder, err := bob.NewBuilder(&bob.BuilderOptions{
		DockerHost:    "unix:///var/run/docker.sock",
		DockerVersion: "1.41",
		Organisation:  "rgynn",
		Timeout:       time.Minute * 5,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := builder.Run("klottr", "626689284788951e300dd5847cef859d711d2266", "latest"); err != nil {
		log.Fatal(err)
	}
}
