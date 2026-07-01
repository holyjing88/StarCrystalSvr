# StarCrystal 开发用 MySQL 镜像（基于官方 mysql:8.4）
FROM mysql:8.4
LABEL org.starcrystal.component="mysql" \
      org.starcrystal.version="8.4"

ENV MYSQL_DATABASE=starcrystal_auth

# 配置、初始化 SQL、备份脚本均打入镜像；数据与备份目录由 docker run 挂载到宿主磁盘
RUN microdnf install -y cronie gzip \
    && microdnf clean all

COPY conf.d/ /etc/mysql/conf.d/
COPY docker-entrypoint-initdb.d/ /docker-entrypoint-initdb.d/
COPY scripts/ /opt/starcrystal/mysql-scripts/
COPY crontab /etc/cron.d/starcrystal-mysql-backup
COPY docker-entrypoint.sh /usr/local/bin/starcrystal-mysql-entrypoint.sh

RUN chmod 0644 /etc/cron.d/starcrystal-mysql-backup \
    && chmod +x /usr/local/bin/starcrystal-mysql-entrypoint.sh \
    && chmod +x /opt/starcrystal/mysql-scripts/*.sh

ENTRYPOINT ["starcrystal-mysql-entrypoint.sh"]
CMD ["mysqld"]
