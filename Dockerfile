# multi-stage Dockerfile to build final minimal image with exrex dependency

# stage 1: build the Go binary
FROM golang:1.25-bookworm AS builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_TIME} -X main.builtBy=forgejo-actions" \
    -o /out/traefik-opnsense-sync ./cmd/traefik-opnsense-sync

# stage 2: install exrex python package and create executable shim for it
FROM python:3.11-slim-bookworm AS exrex-installer
ENV PIP_NO_CACHE_DIR=1 PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PIP_DISABLE_PIP_VERSION_CHECK=1 PIP_ROOT_USER_ACTION=ignore

WORKDIR /exrex

RUN pip install --no-compile --target ./ exrex==0.12.0 && \
    install -m 0755 /dev/stdin exrex <<EOF
#!/usr/bin/python3
import sys
from exrex import __main__
if __name__ == '__main__':
    sys.exit(__main__())
EOF


# stage 3: final minimal image (not using scratch due to exrex python dependency)
FROM gcr.io/distroless/python3-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=exrex-installer /exrex/exrex.py /exrex/exrex ./
COPY --from=builder /out/traefik-opnsense-sync ./

ENTRYPOINT ["./traefik-opnsense-sync"]
