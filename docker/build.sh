#!/usr/bin/env bash
# build, tag, and push docker images

# exit if a command fails
set -o errexit

# go docker image tag to use
tag="${TAG:-latest}"

# if no registry is provided, tag image as "local" registry
registry="${REGISTRY:-local}"

# set image name
image_name="roulette"

# set image version
image_version="latest"

# platforms to build for
platforms="linux/amd64"
platforms+=",linux/arm"
platforms+=",linux/arm64"
platforms+=",linux/ppc64le"

# copy native image to local image repository
docker buildx build \
                    --build-arg TAG="${tag}" \
                    -t "${registry}/${image_name}:${image_version}" \
                    -f Dockerfile . \
                    --load

# push image to remote registry
docker buildx build --platform "${platforms}" \
                    --build-arg TAG="${tag}" \
                    -t "${registry}/${image_name}:${image_version}" \
                    -f Dockerfile . \
                    --push
