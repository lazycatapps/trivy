# syntax=docker/dockerfile:1

ARG commit_id=dev
ARG TRIVY_VERSION=0.67.2

FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy only necessary source files
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o trivy-web-server ./cmd/server

FROM alpine:latest AS runtime-base

ARG commit_id
ARG TRIVY_VERSION
ENV COMMIT_ID=${commit_id}

# Install trivy, docker-cli and configure non-root user
RUN apk add --no-cache ca-certificates wget tar docker-cli \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S nonroot -G nonroot \
    && wget -q https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz \
    && tar -xzf trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz -C /usr/local/bin trivy \
    && rm trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz \
    && chmod +x /usr/local/bin/trivy \
    && apk del wget tar

WORKDIR /app

# Prepare config directory and trivy cache directory with restrictive permissions
RUN mkdir -p /configs /reports /home/nonroot/.cache/trivy \
    && chown -R 65532:65532 /configs /reports /home/nonroot \
    && chmod 700 /configs /reports \
    && chmod 755 /home/nonroot/.cache/trivy

USER 65532:65532

ENV HOME=/home/nonroot

# Download Trivy vulnerability databases (before switching to non-root user)
RUN trivy image --download-db-only --cache-dir /home/nonroot/.cache/trivy \
    && trivy image --download-java-db-only --cache-dir /home/nonroot/.cache/trivy

EXPOSE 8080

ENTRYPOINT ["./trivy-web-server"]

FROM runtime-base AS dev

# Copy pre-built binary from local build context
COPY --chown=65532:65532 trivy-web-server /app/trivy-web-server

FROM runtime-base AS prod

# Copy binary from builder stage for production image
COPY --from=builder --chown=65532:65532 /app/trivy-web-server /app/trivy-web-server
