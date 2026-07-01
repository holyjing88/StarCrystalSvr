#!/bin/bash
# StarCrystal MySQL：启动备份 crond，再交给官方 entrypoint
set -e

mkdir -p /backups /var/log/mysql
chmod 1777 /backups 2>/dev/null || true

cat >/etc/mysql/backup.env <<EOF
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD:-jgyjgyjgy}
MYSQL_DATABASE=${MYSQL_DATABASE:-starcrystal_auth}
BACKUP_ROOT=/backups
KEEP=${MYSQL_BACKUP_KEEP:-14}
EOF
chmod 600 /etc/mysql/backup.env

if command -v crond >/dev/null 2>&1; then
  if ! pgrep -x crond >/dev/null 2>&1; then
    crond -b -l 2 2>/dev/null || crond
  fi
  echo "[starcrystal-mysql] backup crond started (daily 03:00 -> /backups)"
fi

exec /usr/local/bin/docker-entrypoint.sh "$@"
