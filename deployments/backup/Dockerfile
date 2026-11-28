# syntax=docker/dockerfile:1
# Nightly database backup to S3 (docker-compose.yml, profile "backup"):
# pg_dump of the same major version as the server, plus the AWS CLI.
FROM postgres:18.7-alpine3.24@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71
# The AWS CLI version follows the pinned Alpine release.
# hadolint ignore=DL3018
RUN apk add --no-cache aws-cli \
 && aws --version \
 && mkdir /work && chown 999:999 /work
COPY --chmod=0755 backup.sh /usr/local/bin/messora-backup
# The root filesystem is read-only; the AWS CLI keeps its cache in HOME.
ENV HOME=/tmp
USER 999:999
ENTRYPOINT ["/usr/local/bin/messora-backup"]
CMD ["run"]
