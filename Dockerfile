# This Dockerfile utilizes a multi-stage build
ARG ALPINE_VERSION=3.20

FROM golang:1.24-alpine$ALPINE_VERSION AS builder
ARG TARGETARCH
ARG TARGETOS
ARG VERSION

# Install necessary build tools
RUN apk --update add make bash curl git

# Switch to root dir (default for go is /go)
WORKDIR /

# Copy everything into build container
COPY . .

# Build the application
RUN VERSION=$VERSION make build/$TARGETOS-$TARGETARCH

# Now in 2nd build stage
FROM library/alpine:$ALPINE_VERSION
ARG TARGETARCH
ARG TARGETOS

# SSL and quality-of-life tools
RUN apk --update add bash curl ca-certificates && update-ca-certificates

# Copy bin & WASM
COPY --from=builder /build/go-svc-template-$TARGETOS-$TARGETARCH /go-svc-template

RUN chmod +x /go-svc-template

CMD ["/go-svc-template"]
