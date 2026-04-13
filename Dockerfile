###############################################################################
# BEGIN build-stage
# Compile the binary
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26.2 AS build-stage

ARG BUILDPLATFORM
ARG TARGETARCH

WORKDIR /app

COPY vendor ./vendor

COPY go.mod go.sum ./

COPY hack ./hack

COPY cmd ./cmd

COPY pkg ./pkg

RUN GOOS=linux GOARCH="${TARGETARCH}" hack/build.sh

#
# END build-stage
###############################################################################

###############################################################################
# BEGIN final-stage
# Create final docker image
FROM scratch AS final-stage

WORKDIR /

COPY --from=build-stage /app/bin/p3 /

ENTRYPOINT ["/p3"]

#
# END final-stage
###############################################################################
