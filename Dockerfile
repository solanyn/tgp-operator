# Build stage
FROM golang:1.24-alpine AS builder

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates git

WORKDIR /workspace

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the manager binary with security hardening and optimizations
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -trimpath \
    -o manager cmd/manager/main.go

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

# Copy ca-certificates for HTTPS requests to cloud providers
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /workspace/manager /manager

# Use non-root user
USER nonroot:nonroot

ENTRYPOINT ["/manager"]