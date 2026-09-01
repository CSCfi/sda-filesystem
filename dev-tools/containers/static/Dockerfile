ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

COPY --from=hairyhenderson/gomplate:stable@sha256:991ce1b8a1307fa32f9b8050a7b339697464664236cfddb1c4079df4aa5f1cd4 /gomplate /bin/gomplate
COPY ./dev-tools/containers/static/configuration.tmpl /configuration.tmpl

RUN mkdir /static

CMD ["gomplate", "-f", "/configuration.tmpl", "-o", "/static/configuration.json"]
