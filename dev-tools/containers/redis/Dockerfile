ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}redis:8.4.5-alpine3.22@sha256:bd4a0d37e7cd830117ffec9329052b4a1887afa060c265e1768f82b177ff6f43

COPY ./dev-tools/containers/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD [ "redis-server", "/usr/local/etc/redis/redis.conf" ]
