#!/bin/sh
# Bucket bootstrap (EPIC-018). Idempotent: safe to run on every `compose up`.
set -eu

mc alias set icyre "http://minio:9000" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null

# Media bucket is private: no anonymous access, clients only get signed URLs.
mc mb --ignore-existing icyre/icyre-media
mc anonymous set none icyre/icyre-media >/dev/null

echo "bucket icyre-media ready"
