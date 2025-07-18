FROM docker.io/library/alpine:3.15 AS runtime

ENTRYPOINT ["appcat"]

RUN \
  apk add --update --no-cache \
    bash \
    ca-certificates \
    curl

RUN \
  mkdir /.cache && chmod -R g=u /.cache

COPY appcat /usr/local/bin/

RUN chmod a+x /usr/local/bin/appcat

USER 65532:0
