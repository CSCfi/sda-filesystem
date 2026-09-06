ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}redis:8.4.6-alpine3.22@sha256:12da49daa000c2be4d55118574f889c8cc298140ecf343d04f345c818f227823

COPY ./dev-tools/containers/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD [ "redis-server", "/usr/local/etc/redis/redis.conf" ]
