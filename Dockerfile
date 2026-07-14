# Pre-built artefacts are copied from the host by build.sh before this runs.
# No build tools are needed inside the image.
#
# Build flow:
#   1. build.sh compiles the Go binary  →  ./image-roundup  (linux/amd64, static)
#   2. build.sh builds the React SPA   →  ./frontend/dist/
#   3. docker buildx copies both into this distroless image and pushes.

FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy the pre-built Go binary and React SPA
COPY image-roundup      /image-roundup
COPY frontend/dist      ./static

# STATIC_DIR is read by the Go server to locate the SPA files.
# DATA_DIR is intentionally unset here — the server runs memory-only when
# DATA_DIR is absent, which is correct for local `docker run`.
# In Kubernetes, DATA_DIR=/data is set via the Deployment env and backed by PVC.
ENV STATIC_DIR=/app/static

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/image-roundup"]
