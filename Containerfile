# --- Builder stage ---
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

ARG TARGETOS=linux
ARG TARGETARCH
ENV CGO_ENABLED=0

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -trimpath \
      -buildvcs=false \
      -ldflags="-s -w" \
      -o /out/search-synthesis-proxy \
      ./cmd/proxy

# --- Runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /out/search-synthesis-proxy /search-synthesis-proxy

ENV PROXY_PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/search-synthesis-proxy"]
