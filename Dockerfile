FROM docker.io/library/alpine:3.15 as runtime

ENTRYPOINT ["appcat-apiserver"]

RUN \
  apk add --update --no-cache \
    bash \
    ca-certificates \
    curl

RUN \
  mkdir /.cache && chmod -R g=u /.cache

COPY pkg/apiserver /usr/local/bin/

RUN chmod a+x /usr/local/bin/apiserver

USER 65532:0
