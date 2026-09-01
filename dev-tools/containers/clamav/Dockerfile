ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}clamav/clamav:1.5.2-debian@sha256:14e2e0805c6a5ff6728ea591c05565e0f0954d93e8701919012c3a8c44e20674
ARG PROXY_URL

RUN sed -i '/^DatabaseMirror /s/^/#/' /etc/clamav/freshclam.conf
RUN sed -i "\$aPrivateMirror ${PROXY_URL}/clamav" /etc/clamav/freshclam.conf

RUN sed -i 's|^MaxFileSize .*|MaxFileSize 35M|' /etc/clamav/clamd.conf
RUN sed -i "\$aTCPAddr 0.0.0.0" /etc/clamav/clamd.conf
