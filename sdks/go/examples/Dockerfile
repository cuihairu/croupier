ARG GO_IMAGE=golang:1.26.1
ARG RUNTIME_IMAGE=gcr.io/distroless/base-debian12:nonroot

FROM ${GO_IMAGE} AS build
WORKDIR /src

ARG EXAMPLE_DIR=basic

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOFLAGS=-mod=mod go build -o /out/example ./examples/${EXAMPLE_DIR}

FROM ${RUNTIME_IMAGE}
WORKDIR /app
COPY --from=build /out/example /app/example
ENTRYPOINT ["/app/example"]
