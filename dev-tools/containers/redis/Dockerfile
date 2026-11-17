ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}redis:8.10.3-alpine3.23@sha256:7958cfbb9d152f87dadfbc89f1b0b15938f0b81ff8982feb327476f4911935c3

COPY ./dev-tools/containers/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD [ "redis-server", "/usr/local/etc/redis/redis.conf" ]
