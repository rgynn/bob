# Bob

## Experimental package to:

1. Git clone repo
2. Archive it
3. Build docker image
4. Push docker image to a repo


## Install as cli:

```sh
go install github.com/rgynn/bob/cmd/bob@v0.0.2
```

## Usage:

```sh
Usage of bob:
  -commit string
        git commit to checkout from repository and tag docker image with
  -docker-image string
        docker image to push
  -git-repo string
        git repository to checkout
  -p string
        docker repository password to push with
  -tags string
        additional tags to push docker image with
  -timeout duration
        timeout for job (default 5m0s)
  -u string
        docker repository user to push with (default "00000000-0000-0000-0000-000000000000")
```

## Using said cli:

```sh
bob -git-repo $GIT_REPOSITORY -commit $GIT_COMMIT_HASH -docker-image $DOCKER_REGISTRY/$DOCKER_IMAGE -p $DOCKER_REGISTRY_AUTH_TOKEN
```