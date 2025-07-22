FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk --no-cache add ca-certificates git

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -trimpath \
    -a \
    -installsuffix cgo \
    -o manager cmd/manager/main.go

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/manager /manager

USER nonroot:nonroot

ENTRYPOINT ["/manager"]