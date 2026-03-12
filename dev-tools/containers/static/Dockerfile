FROM alpine:3.23

COPY --from=hairyhenderson/gomplate:stable /gomplate /bin/gomplate
COPY ./dev-tools/containers/static/configuration.tmpl /configuration.tmpl

RUN mkdir /static

CMD ["gomplate", "-f", "/configuration.tmpl", "-o", "/static/configuration.json"]
