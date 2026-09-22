ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}redis:8.4.7-alpine3.22@sha256:6935373cd72add2b46b624158d841afdeef5cce1dc6fe9e6d0cb94bb677169ad

COPY ./dev-tools/containers/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD [ "redis-server", "/usr/local/etc/redis/redis.conf" ]
