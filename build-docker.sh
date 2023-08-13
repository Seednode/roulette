#!/usr/bin/env bash
docker run -it --rm -v "$(pwd)":/code golang:alpine /bin/ash -c 'apk update && apk add bash && cd /code && ./build.sh'
