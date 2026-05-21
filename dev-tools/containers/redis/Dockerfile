ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}redis:8.4.3-alpine3.22

COPY ./dev-tools/containers/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD [ "redis-server", "/usr/local/etc/redis/redis.conf" ]
