# StarCrystal 开发用 Redis 镜像（基于官方 redis:7-alpine + 容器内定时备份）
FROM redis:7-alpine
LABEL org.starcrystal.component="redis" \
      org.starcrystal.version="7-alpine"

RUN apk add --no-cache bash dcron

COPY scripts/ /opt/starcrystal/redis-scripts/
COPY redis.conf /etc/redis/redis.conf
COPY crontab /etc/crontabs/root
COPY docker-entrypoint.sh /usr/local/bin/starcrystal-redis-entrypoint.sh
RUN chmod +x /usr/local/bin/starcrystal-redis-entrypoint.sh \
    && chmod +x /opt/starcrystal/redis-scripts/*.sh

ENTRYPOINT ["starcrystal-redis-entrypoint.sh"]
CMD ["redis-server", "/etc/redis/redis.conf"]
