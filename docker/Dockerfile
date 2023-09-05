# set app name
ARG app=roulette

# create build stage
ARG TAG
FROM --platform=$BUILDPLATFORM golang:$TAG AS build
ARG app

# install dependencies
RUN apk add --update-cache git upx

# clone
RUN git clone https://git.seedno.de/seednode/$app /src/$app

# build and compress the binary
WORKDIR /src/$app
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -ldflags "-s -w" -o $app \
    && upx --best --lzma $app \
    && chmod 500 $app

# set up final stage
FROM scratch
ARG app

# copy in user info
COPY --chown=root:root --chmod=0400 passwd /etc/passwd

# run as nonroot
USER nonroot

# copy in binary
COPY --from=build --chown=root:root --chmod=0005 /src/$app/$app /$app

# copy in time zone info
COPY --from=build --chown=root:root --chmod=0004 /usr/local/go/lib/time/zoneinfo.zip /

# load time zone info
ENV ZONEINFO=/zoneinfo.zip

# listen on an unprivileged port
EXPOSE 8080

# run application
ENTRYPOINT ["/roulette"]
CMD ["-r","/data"]
