# syntax=docker/dockerfile:1

# 1. Build the web client.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# 2. Build one static binary with the client embedded.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/fond ./cmd/fond \
 && mkdir -p /out/data

# 3. Ship nothing but the binary and CA certificates.
FROM scratch
LABEL org.opencontainers.image.title="Fond" \
      org.opencontainers.image.description="A self-hosted recipe keeper" \
      org.opencontainers.image.source="https://github.com/au12/fond" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/fond /fond
COPY --from=build --chown=99:100 /out/data /data
ENV FOND_DATA=/data PORT=8080 PUID=99 PGID=100
EXPOSE 8080
VOLUME /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/fond", "healthcheck"]
ENTRYPOINT ["/fond"]
