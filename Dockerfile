FROM docker.io/library/alpine:3.15 as runtime

ENTRYPOINT ["appcat-apiserver"]
CMD ["api-server"]

RUN \
  apk add --update --no-cache \
    bash \
    ca-certificates \
    curl

RUN \
  mkdir /.cache && chmod -R g=u /.cache

COPY appcat-apiserver /usr/local/bin/

RUN chmod a+x /usr/local/bin/appcat-apiserver

USER 65532:0
