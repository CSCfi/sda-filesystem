ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}alpine:3.24

COPY --from=hairyhenderson/gomplate:stable /gomplate /bin/gomplate
COPY ./dev-tools/containers/static/configuration.tmpl /configuration.tmpl

RUN mkdir /static

CMD ["gomplate", "-f", "/configuration.tmpl", "-o", "/static/configuration.json"]
