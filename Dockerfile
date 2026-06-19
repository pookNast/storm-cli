# Multi-stage build: compile on golang:1.24, run on distroless/static (no CGO)
FROM golang:1.24 AS build

WORKDIR /src

# Copy dependency manifests first for layer caching
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=local go mod download

# Copy source and build static binary
COPY . .
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /storm ./cmd/storm

# ---

# Run stage: minimal distroless image (no shell, no package manager)
FROM gcr.io/distroless/static-debian12:nonroot AS run

COPY --from=build /storm /storm

# out/ is mounted at runtime; no data baked into image
ENTRYPOINT ["/storm"]
CMD ["--help"]
