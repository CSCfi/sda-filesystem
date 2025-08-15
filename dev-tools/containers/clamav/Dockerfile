FROM clamav/clamav:1.4
ARG PROXY_URL

RUN sed -i '/^DatabaseMirror /s/^/#/' /etc/clamav/freshclam.conf
RUN sed -i "\$aPrivateMirror ${PROXY_URL}/clamav" /etc/clamav/freshclam.conf

RUN sed -i 's|^MaxFileSize .*|MaxFileSize 35M|' /etc/clamav/clamd.conf
RUN sed -i "\$aTCPAddr 0.0.0.0" /etc/clamav/clamd.conf
