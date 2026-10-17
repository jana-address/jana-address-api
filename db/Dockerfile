# Official PostgreSQL 18.6 (alpine), pinned by multi-arch index digest (resolved 2026-10-17).
ARG PG_IMAGE=postgres:18.6-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873
FROM ${PG_IMAGE}

# The upstream entrypoint uses gosu only to drop root -> postgres. We start as postgres,
# so gosu is never called. Removing it drops ~25 Go stdlib CVEs (gosu 1.19 is built with
# go1.24.6) and the image no longer runs anything as root.
RUN rm -f /usr/local/bin/gosu
USER postgres
