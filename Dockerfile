# multi-stage Dockerfile to build final minimal image with exrex dependency

# stage 1: install exrex python package and create executable shim for it
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


# stage 3: final minimal image (not using scrarch due to exrex python dependency)
FROM gcr.io/distroless/python3-debian12:nonroot AS final
ARG TARGETPLATFORM

WORKDIR /app

COPY --from=exrex-installer /exrex/exrex.py /exrex/exrex ./
COPY $TARGETPLATFORM/traefik-opnsense-sync ./

ENTRYPOINT ["./traefik-opnsense-sync"]
