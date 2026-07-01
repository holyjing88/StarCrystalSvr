#!/usr/bin/env bash
# shellcheck disable=SC2034
# Source from docker_*.sh after starcrystal-config.sh
export LANG="${LANG:-C.UTF-8}"

: "${SCRIPT_DIR:?}"
: "${RELEASE_ROOT:?}"

sc_docker_load_linux_host_env() {
  [[ -n "${REPO_ROOT:-}" ]] || return 0
  local f="${REPO_ROOT}/tools/linux-host.local.env"
  [[ -f "$f" ]] || return 0
  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" ]] && continue
    [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    [[ -n "${!key+x}" ]] && continue
    val="${BASH_REMATCH[2]}"
    val="${val%\"}"; val="${val#\"}"
    val="${val%\'}"; val="${val#\'}"
    export "$key=$val"
  done <"$f"
}

sc_docker_resolve_mirror_save_dir() {
  if [[ -n "${DOCKER_MIRROR_SAVE_DIR:-}" ]]; then
    printf '%s\n' "$DOCKER_MIRROR_SAVE_DIR"
    return 0
  fi
  if [[ -d "${PWD}/mirror_save" ]]; then
    printf '%s\n' "${PWD}/mirror_save"
  else
    printf '%s\n' "${SCRIPT_DIR}/mirror_save"
  fi
}

DOCKER_MIRROR_SAVE_DIR="$(sc_docker_resolve_mirror_save_dir)"
export DOCKER_MIRROR_SAVE_DIR

DOCKER_MYSQL_CONTAINER="${DOCKER_MYSQL_CONTAINER:-starcrystal-dev-mysql}"
DOCKER_REDIS_CONTAINER="${DOCKER_REDIS_CONTAINER:-starcrystal-dev-redis}"
DOCKER_RELEASE_CONTAINER="${DOCKER_RELEASE_CONTAINER:-starcrystal-dev-release}"
DOCKER_MYSQL_IMAGE_REPO="${DOCKER_MYSQL_IMAGE_REPO:-starcrystal/mysql}"
DOCKER_MYSQL_IMAGE_TAG="${DOCKER_MYSQL_IMAGE_TAG:-8.4}"
DOCKER_MYSQL_IMAGE="${DOCKER_MYSQL_IMAGE:-${DOCKER_MYSQL_IMAGE_REPO}:${DOCKER_MYSQL_IMAGE_TAG}}"
DOCKER_REDIS_IMAGE_REPO="${DOCKER_REDIS_IMAGE_REPO:-starcrystal/redis}"
DOCKER_REDIS_IMAGE_TAG="${DOCKER_REDIS_IMAGE_TAG:-7-alpine}"
DOCKER_REDIS_IMAGE="${DOCKER_REDIS_IMAGE:-${DOCKER_REDIS_IMAGE_REPO}:${DOCKER_REDIS_IMAGE_TAG}}"
DOCKER_RELEASE_IMAGE_REPO="${DOCKER_RELEASE_IMAGE_REPO:-starcrystal/release}"
DOCKER_RELEASE_IMAGE_TAG="${DOCKER_RELEASE_IMAGE_TAG:-bundle}"
DOCKER_RELEASE_IMAGE="${DOCKER_RELEASE_IMAGE:-${DOCKER_RELEASE_IMAGE_REPO}:${DOCKER_RELEASE_IMAGE_TAG}}"
DOCKER_REDIS_CONTEXT="${DOCKER_REDIS_CONTEXT:-$SCRIPT_DIR/redis}"
DOCKER_MYSQL_CONTEXT="${DOCKER_MYSQL_CONTEXT:-$SCRIPT_DIR/mysql}"
DOCKER_RELEASE_CONTEXT="${DOCKER_RELEASE_CONTEXT:-$SCRIPT_DIR/release}"

DOCKER_MYSQL_TAR_NAME="${DOCKER_MYSQL_TAR_NAME:-starcrystal-mysql-8.4.tar}"
DOCKER_REDIS_TAR_NAME="${DOCKER_REDIS_TAR_NAME:-starcrystal-redis-7-alpine.tar}"
DOCKER_RELEASE_TAR_NAME="${DOCKER_RELEASE_TAR_NAME:-starcrystal-release-bundle.tar}"
DOCKER_MYSQL_TAR="${DOCKER_MYSQL_TAR:-$DOCKER_MIRROR_SAVE_DIR/mysql/$DOCKER_MYSQL_TAR_NAME}"
DOCKER_REDIS_TAR="${DOCKER_REDIS_TAR:-$DOCKER_MIRROR_SAVE_DIR/redis/$DOCKER_REDIS_TAR_NAME}"
DOCKER_RELEASE_TAR="${DOCKER_RELEASE_TAR:-$DOCKER_MIRROR_SAVE_DIR/release/$DOCKER_RELEASE_TAR_NAME}"

DOCKER_REDIS_BASE_IMAGE="${DOCKER_REDIS_BASE_IMAGE:-redis:7-alpine}"
DOCKER_MYSQL_BASE_IMAGE="${DOCKER_MYSQL_BASE_IMAGE:-mysql:8.4}"
DOCKER_REDIS_BASE_TAR="${DOCKER_REDIS_BASE_TAR:-$DOCKER_MIRROR_SAVE_DIR/redis/redis-7-alpine-base.tar}"
DOCKER_MYSQL_BASE_TAR="${DOCKER_MYSQL_BASE_TAR:-$DOCKER_MIRROR_SAVE_DIR/mysql/mysql-8.4-base.tar}"
DOCKER_RELEASE_BASE_IMAGE="${DOCKER_RELEASE_BASE_IMAGE:-alpine:3.20}"
DOCKER_DOCKERFILE_FRONTEND_IMAGE="${DOCKER_DOCKERFILE_FRONTEND_IMAGE:-docker/dockerfile:1}"
DOCKER_RELEASE_BASE_TAR="${DOCKER_RELEASE_BASE_TAR:-$DOCKER_MIRROR_SAVE_DIR/release/alpine-3.20-base.tar}"
DOCKER_DOCKERFILE_FRONTEND_TAR="${DOCKER_DOCKERFILE_FRONTEND_TAR:-$DOCKER_MIRROR_SAVE_DIR/dockerfile-1-base.tar}"

sc_docker_load_linux_host_env
DOCKER_MYSQL_PORT="${DOCKER_MYSQL_PORT:-3306}"
DOCKER_REDIS_PORT="${DOCKER_REDIS_PORT:-6379}"

sc_docker_redis_stage_scripts() {
  local stage="$DOCKER_REDIS_CONTEXT/scripts"
  local src="$SCRIPT_DIR/../scripts/dbscripts/redis"
  mkdir -p "$stage"
  local f
  for f in redis-backup.sh redis-common.sh; do
    if [[ ! -f "$src/$f" ]]; then
      echo "[docker_redis] 缺少备份脚本: $src/$f" >&2
      return 1
    fi
    cp -f "$src/$f" "$stage/$f"
    chmod +x "$stage/$f"
  done
}

sc_docker_container_cron_start() {
  local name="$1" prep="${2:-}"
  sc_docker_container_running "$name" || return 0
  docker exec "$name" sh -c "
    ${prep}
    if command -v crond >/dev/null 2>&1; then
      if ! pgrep -x crond >/dev/null 2>&1 && ! pidof crond >/dev/null 2>&1; then
        crond -b -l 2 2>/dev/null || crond
      fi
    fi
  " 2>/dev/null || true
}

sc_docker_container_cron_stop() {
  local name="$1"
  sc_docker_container_running "$name" || return 0
  docker exec "$name" sh -c 'pkill -x crond 2>/dev/null || true' 2>/dev/null || true
}

sc_docker_redis_backup_start() {
  sc_docker_container_cron_start "${1:-$DOCKER_REDIS_CONTAINER}" \
    'mkdir -p /data/backups'
  echo "[docker_redis] backup cron started (in-container, daily 03:00 -> /data/backups)"
}

sc_docker_redis_backup_stop() {
  sc_docker_container_cron_stop "${1:-$DOCKER_REDIS_CONTAINER}"
  echo "[docker_redis] backup cron stopped"
}

sc_docker_redis_wait_ready() {
  local container="${1:-$DOCKER_REDIS_CONTAINER}"
  local configured="${2:-$DOCKER_REDIS_PORT}"
  local data="${3:-$DOCKER_REDIS_DATA}"
  local port
  port="$(sc_docker_resolve_host_port "$container" 6379 "$configured" docker_redis)"
  echo "[docker_redis] waiting port $port ..."
  if sc_docker_wait_tcp 127.0.0.1 "$port" 30; then
    sc_docker_redis_backup_start "$container"
    echo "[docker_redis] OK ($container, data=$data)"
    return 0
  fi
  if sc_docker_redis_ping "$container"; then
    sc_docker_redis_backup_start "$container"
    echo "[docker_redis] OK ($container, data=$data, host TCP :$port failed but redis-cli PING ok)"
    return 0
  fi
  echo "[docker_redis] ERROR: port $port not ready after 30s" >&2
  docker logs --tail 30 "$container" 2>&1 || true
  return 1
}
DOCKER_RELEASE_PORT="${DOCKER_RELEASE_PORT:-8080}"
DOCKER_MYSQL_ROOT_PASSWORD="${DOCKER_MYSQL_ROOT_PASSWORD:-jgyjgyjgy}"
DOCKER_DATA_DIR="${DOCKER_DATA_DIR:-$REPO_ROOT/.docker-data}"
# 生产默认宿主路径（build/start 时自动 mkdir；数据仅通过 volume 挂载，不打入镜像）
DOCKER_MYSQL_DATA="${DOCKER_MYSQL_DATA:-/app/mysql/db}"
# 备份目录与数据目录分离（勿放在 /app/mysql/db 内）
DOCKER_MYSQL_BACKUP_DIR="${DOCKER_MYSQL_BACKUP_DIR:-/app/mysql/db_backup}"
DOCKER_REDIS_DATA="${DOCKER_REDIS_DATA:-/app/redis/db}"
DOCKER_REDIS_BACKUP_DIR="${DOCKER_REDIS_BACKUP_DIR:-/app/redis/db_backup}"
DOCKER_RELEASE_SVR_BIN="${DOCKER_RELEASE_SVR_BIN:-}"
DOCKER_RELEASE_SVR_DIR="${DOCKER_RELEASE_SVR_DIR:-/app/starcrystalsvr}"

sc_docker_ensure_host_data_dirs() {
  mkdir -p "$DOCKER_MYSQL_DATA" "$DOCKER_MYSQL_BACKUP_DIR" "$DOCKER_REDIS_DATA" "$DOCKER_REDIS_BACKUP_DIR"
  chmod 755 /app /app/mysql /app/redis 2>/dev/null || true
}

sc_docker_mysql_has_data() {
  local dir="$1" f base
  [[ -f "$dir/ibdata1" ]] || [[ -d "$dir/mysql" ]] && return 0
  for f in "$dir"/*; do
    [[ -e "$f" ]] || return 1
    base="$(basename "$f")"
    [[ "$base" == "start.log" ]] && continue
    [[ "$base" == "backups" ]] && continue
    return 0
  done
  return 1
}

sc_docker_redis_has_data() {
  local dir="$1" f base
  [[ -f "$dir/dump.rdb" ]] || [[ -f "$dir/appendonly.aof" ]] && return 0
  for f in "$dir"/*; do
    [[ -e "$f" ]] || return 1
    base="$(basename "$f")"
    [[ "$base" == "start.log" ]] && continue
    [[ "$base" == "backups" ]] && continue
    return 0
  done
  return 1
}

sc_docker_note_mysql_data() {
  local dir="$1"
  if sc_docker_mysql_has_data "$dir"; then
    echo "[docker_mysql] using existing data in $dir (skip re-init)"
  else
    echo "[docker_mysql] empty data dir $dir — first start will initialize schema"
  fi
}

sc_docker_note_redis_data() {
  local dir="$1"
  if sc_docker_redis_has_data "$dir"; then
    echo "[docker_redis] using existing data in $dir"
  else
    echo "[docker_redis] empty data dir $dir — Redis will create dump.rdb on save"
  fi
}

sc_docker_container_bind_source() {
  local container="$1" mountpoint="$2"
  docker inspect -f '{{range .Mounts}}{{if eq .Destination "'"$mountpoint"'"}}{{.Source}}{{end}}{{end}}' "$container" 2>/dev/null
}

sc_docker_warn_volume_mismatch() {
  local label="$1" container="$2" expected="$3" mountpoint="$4"
  local current
  sc_docker_container_exists "$container" || return 0
  current="$(sc_docker_container_bind_source "$container" "$mountpoint")"
  [[ -z "$current" || "$current" == "$expected" ]] && return 0
  echo "[$label] WARN: container bind $mountpoint -> $current (expected $expected)" >&2
  echo "[$label]       保留现有数据请勿覆盖；改挂载需: docker rm -f $container 后再 start" >&2
}

sc_docker_mysql_stage_scripts() {
  local stage="$DOCKER_MYSQL_CONTEXT/scripts"
  local src="$SCRIPT_DIR/../scripts/dbscripts/mysql"
  mkdir -p "$stage"
  local f
  for f in mysql-backup.sh mysql-common.sh; do
    if [[ ! -f "$src/$f" ]]; then
      echo "[docker_mysql] 缺少备份脚本: $src/$f" >&2
      return 1
    fi
    cp -f "$src/$f" "$stage/$f"
    chmod +x "$stage/$f"
  done
}

sc_docker_mysql_backup_start() {
  sc_docker_container_cron_start "${1:-$DOCKER_MYSQL_CONTAINER}" \
    'mkdir -p /backups; chmod 0644 /etc/cron.d/starcrystal-mysql-backup 2>/dev/null || true'
  echo "[docker_mysql] backup cron started (in-container, daily 03:00 -> /backups)"
}

sc_docker_mysql_backup_stop() {
  sc_docker_container_cron_stop "${1:-$DOCKER_MYSQL_CONTAINER}"
  echo "[docker_mysql] backup cron stopped"
}

sc_docker_verify_staged_backup_scripts() {
  local kind="$1" ctx f
  case "$kind" in
    mysql)
      ctx="$DOCKER_MYSQL_CONTEXT"
      for f in mysql-backup.sh mysql-common.sh; do
        if [[ ! -f "$ctx/scripts/$f" ]]; then
          echo "[docker] 缺少 staged 备份脚本: $ctx/scripts/$f" >&2
          return 1
        fi
      done
      [[ -f "$ctx/crontab" ]] || { echo "[docker] 缺少 $ctx/crontab" >&2; return 1; }
      ;;
    redis)
      ctx="$DOCKER_REDIS_CONTEXT"
      for f in redis-backup.sh redis-common.sh; do
        if [[ ! -f "$ctx/scripts/$f" ]]; then
          echo "[docker] 缺少 staged 备份脚本: $ctx/scripts/$f" >&2
          return 1
        fi
      done
      [[ -f "$ctx/crontab" ]] || { echo "[docker] 缺少 $ctx/crontab" >&2; return 1; }
      ;;
    *)
      echo "[docker] unknown kind: $kind" >&2
      return 1
      ;;
  esac
  echo "[docker_$kind] staged 备份脚本与 crontab 就绪"
}

sc_docker_verify_image_backup_bundle() {
  local kind="$1" image="$2"
  case "$kind" in
    mysql)
      docker run --rm "$image" sh -c '
        test -x /opt/starcrystal/mysql-scripts/mysql-backup.sh &&
        test -x /opt/starcrystal/mysql-scripts/mysql-common.sh &&
        test -f /etc/cron.d/starcrystal-mysql-backup &&
        grep -q mysql-backup.sh /etc/cron.d/starcrystal-mysql-backup
      '
      ;;
    redis)
      docker run --rm "$image" sh -c '
        test -x /opt/starcrystal/redis-scripts/redis-backup.sh &&
        test -x /opt/starcrystal/redis-scripts/redis-common.sh &&
        test -f /etc/crontabs/root &&
        grep -q redis-backup.sh /etc/crontabs/root
      '
      ;;
    *)
      return 1
      ;;
  esac
}

sc_docker_need() {
  command -v docker >/dev/null 2>&1 || {
    echo "需要 docker 命令（Docker Desktop / Linux docker）" >&2
    return 1
  }
  docker info >/dev/null 2>&1 || {
    echo "docker daemon 未运行" >&2
    return 1
  }
}

sc_docker_container_running() {
  local name="$1"
  docker ps --format '{{.Names}}' 2>/dev/null | grep -Fxq "$name"
}

sc_docker_container_exists() {
  local name="$1"
  docker ps -a --format '{{.Names}}' 2>/dev/null | grep -Fxq "$name"
}

sc_docker_container_published_port() {
  local container="$1" container_port="$2" line host_port key
  line="$(docker port "$container" "${container_port}/tcp" 2>/dev/null | head -1)" || true
  if [[ -n "$line" ]]; then
    host_port="${line##*:}"
    [[ "$host_port" =~ ^[0-9]+$ ]] && { printf '%s\n' "$host_port"; return 0; }
  fi
  key="${container_port}/tcp"
  host_port="$(docker inspect -f '{{range $p,$v := .HostConfig.PortBindings}}{{if eq $p "'"$key"'"}}{{if $v}}{{(index $v 0).HostPort}}{{end}}{{end}}{{end}}' "$container" 2>/dev/null)" || true
  [[ "$host_port" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "$host_port"
}

# 等待宿主机端口：已有容器优先用 docker port 实际映射（避免未 export DOCKER_*_PORT 时等错端口）
sc_docker_resolve_host_port() {
  local container="$1" container_port="$2" configured_port="$3" label="${4:-docker}"
  local published=""
  if sc_docker_container_exists "$container"; then
    published="$(sc_docker_container_published_port "$container" "$container_port" 2>/dev/null || true)"
  fi
  if [[ -z "$published" ]]; then
    published="$configured_port"
  elif [[ "$published" != "$configured_port" ]]; then
    echo "[$label] NOTE: 等待宿主机 :$published（配置 DOCKER_*_PORT=$configured_port；可 export 或写入 tools/linux-host.local.env）" >&2
  fi
  printf '%s\n' "$published"
}

sc_docker_redis_ping() {
  local container="${1:-$DOCKER_REDIS_CONTAINER}"
  sc_docker_container_running "$container" || return 1
  docker exec "$container" redis-cli ping 2>/dev/null | grep -q '^PONG$'
}

sc_docker_wait_release_ready() {
  local container="$1" configured_port="$2" tries="${3:-45}"
  local port i
  port="$(sc_docker_resolve_host_port "$container" 8080 "$configured_port" docker_release)"
  echo "[docker_release] waiting port $port ..."
  for ((i = 1; i <= tries; i++)); do
    if sc_docker_wait_tcp 127.0.0.1 "$port" 1; then
      return 0
    fi
    if sc_docker_container_running "$container"; then
      docker exec "$container" python3 - <<'PY' 2>/dev/null && return 0
import urllib.request
urllib.request.urlopen("http://127.0.0.1:8080/", timeout=1)
PY
    fi
    sleep 1
  done
  return 1
}

sc_docker_print_release_failure_logs() {
  local container="$1" log_dir="$2"
  docker logs --tail 30 "$container" 2>&1 || true
  local f
  for f in starcrystalsvr.stderr.log starcrystalsvr.stdout.log; do
    if [[ -f "$log_dir/$f" ]]; then
      echo "--- $log_dir/$f (last 40 lines) ---"
      tail -40 "$log_dir/$f" 2>/dev/null || true
    fi
  done
}

# 单次 TCP 探测（nc 可能是 Ncat 不支持 -z，失败时继续试 /dev/tcp）
sc_docker_tcp_open() {
  local host="$1" port="$2"
  if command -v nc >/dev/null 2>&1; then
    nc -z -w 1 "$host" "$port" 2>/dev/null && return 0
  fi
  if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
    exec 3<&-
    exec 3>&-
    return 0
  fi
  if command -v timeout >/dev/null 2>&1; then
    timeout 1 bash -c "exec 3<>/dev/tcp/$host/$port" 2>/dev/null && return 0
  fi
  return 1
}

sc_docker_wait_tcp() {
  local host="$1" port="$2" tries="${3:-30}"
  local i
  for ((i = 1; i <= tries; i++)); do
    sc_docker_tcp_open "$host" "$port" && return 0
    sleep 1
  done
  return 1
}

sc_docker_mysql_ping() {
  local container="${1:-$DOCKER_MYSQL_CONTAINER}"
  local pw="${2:-${DOCKER_MYSQL_ROOT_PASSWORD:-jgyjgyjgy}}"
  sc_docker_container_running "$container" || return 1
  docker exec -e "MYSQL_PWD=$pw" "$container" \
    mysql -h127.0.0.1 -P3306 -uroot -e "SELECT 1" >/dev/null 2>&1
}

# start 在前台等待 mysqld 就绪；容器退出则立即失败（避免仅 TCP 探测误报 OK）
sc_docker_wait_mysql_ready() {
  local name="${1:-$DOCKER_MYSQL_CONTAINER}" tries="${2:-60}"
  local i
  for ((i = 1; i <= tries; i++)); do
    if ! sc_docker_container_running "$name"; then
      echo "[docker_mysql] ERROR: 容器 $name 未在运行（可能数据目录无效导致 mysqld 退出）" >&2
      docker logs --tail 40 "$name" 2>&1 || true
      return 1
    fi
    sc_docker_mysql_ping "$name" && return 0
    sleep 1
  done
  return 1
}

sc_docker_dev_auth_dsn() {
  echo "star_auth:star_auth_123456@tcp(127.0.0.1:${DOCKER_MYSQL_PORT})/starcrystal_auth?charset=utf8mb4&parseTime=true&loc=Local"
}

sc_docker_container_auth_dsn() {
  local host="${1:-host.docker.internal}"
  echo "star_auth:star_auth_123456@tcp(${host}:${DOCKER_MYSQL_PORT})/starcrystal_auth?charset=utf8mb4&parseTime=true&loc=Local"
}

sc_docker_image_exists() {
  docker image inspect "$1" >/dev/null 2>&1
}

sc_docker_try_load_base_image() {
  local base_image="$1" base_tar="${2:-}"
  if sc_docker_image_exists "$base_image"; then
    return 0
  fi
  if [[ -n "$base_tar" && -f "$base_tar" ]]; then
    echo "[docker] 从离线 tar 导入基础镜像: $base_tar"
    sc_docker_load_from_mirror "$base_tar" "base-$base_image"
    sc_docker_image_exists "$base_image"
    return $?
  fi
  return 1
}

sc_docker_build_fail_hint() {
  echo "" >&2
  echo "无法构建（本机缺少 Docker Hub 基础镜像且无法联网拉取）。" >&2
  echo "可选方案:" >&2
  echo "  A) 离线成品镜像: bash tools/docker/docker_redis.sh load && bash tools/docker/docker_redis.sh start" >&2
  echo "  B) 离线基础镜像后 build:" >&2
  echo "     bash tools/docker/fetch-docker-base-images.sh load" >&2
  echo "     bash tools/docker/docker_redis.sh build" >&2
  echo "  C) 修复证书/镜像源: bash tools/docker/setup-docker-linux.sh" >&2
  echo "  D) MySQL 卡在 microdnf metadata: bash tools/docker/mysql/fetch-rpms.sh 后重试 build（完全离线）" >&2
  echo "     或确认 DOCKER_BUILD_NETWORK=host；仍卡住可 systemctl restart docker" >&2
  echo "  E) resolve dockerfile:1 401: 已移除 syntax 指令；build 使用 docker build --network=host" >&2
}

sc_docker_prepare_build() {
  export DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-0}"
  export DOCKER_BUILD_NETWORK="${DOCKER_BUILD_NETWORK:-host}"
  sc_docker_try_load_base_image "$DOCKER_DOCKERFILE_FRONTEND_IMAGE" "$DOCKER_DOCKERFILE_FRONTEND_TAR" || true
}

sc_docker_build_image() {
  local image="$1" context="$2" base_image="${3:-}" base_tar="${4:-}"
  sc_docker_prepare_build
  local net="${DOCKER_BUILD_NETWORK:-host}"
  local -a build_args=(--network="$net")
  [[ "${DOCKER_BUILD_PULL:-0}" == "1" ]] || build_args+=(--pull=false)
  echo "[docker] build --network=$net DOCKER_BUILDKIT=$DOCKER_BUILDKIT ${build_args[*]} -t $image"
  if [[ -n "$base_image" ]]; then
    sc_docker_try_load_base_image "$base_image" "$base_tar" || true
    if ! sc_docker_image_exists "$base_image"; then
      sc_docker_build_fail_hint
      return 1
    fi
    echo "[docker] 使用本地基础镜像 $base_image 构建"
    if docker build "${build_args[@]}" -t "$image" "$context"; then return 0; fi
  else
    if docker build "${build_args[@]}" -t "$image" "$context"; then return 0; fi
  fi
  sc_docker_build_fail_hint
  return 1
}

sc_docker_save_to_mirror() {
  local image="$1" tar_path="$2" label="${3:-image}"
  mkdir -p "$(dirname "$tar_path")"
  echo "[$label] saving -> $tar_path"
  docker save -o "$tar_path" "$image"
  ls -lh "$tar_path"
}

sc_docker_auto_save_mirror() {
  local image="$1" tar_path="$2" label="${3:-image}"
  if [[ "${SKIP_MIRROR_SAVE:-0}" == "1" ]]; then
    echo "[$label] skip mirror_save (SKIP_MIRROR_SAVE=1)"
    return 0
  fi
  sc_docker_save_to_mirror "$image" "$tar_path" "$label"
  echo "[$label] 已导出到 mirror_save: $tar_path"
}

sc_docker_find_mirror_tar() {
  local tar_path="$1"
  local name subdir candidate base
  name="$(basename "$tar_path")"
  subdir="$(basename "$(dirname "$tar_path")")"
  # load 优先：当前目录 ./mirror_save，其次脚本目录 tools/docker/mirror_save
  for base in "${PWD}/mirror_save" "${SCRIPT_DIR}/mirror_save" "${DOCKER_MIRROR_SAVE_DIR:-}"; do
    [[ -z "$base" ]] && continue
    candidate="${base}/${subdir}/${name}"
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  [[ -f "$tar_path" ]] && { printf '%s\n' "$tar_path"; return 0; }
  return 1
}

sc_docker_load_from_mirror() {
  local tar_path="$1" label="${2:-image}" resolved
  if ! resolved="$(sc_docker_find_mirror_tar "$tar_path")"; then
    echo "[$label] 未找到 tar: $tar_path" >&2
    echo "[$label] 已查找: ${PWD}/mirror_save/${subdir}/${name}、${SCRIPT_DIR}/mirror_save/${subdir}/${name}" >&2
    return 1
  fi
  if [[ "$resolved" != "$tar_path" ]]; then
    echo "[$label] 使用 mirror_save: $resolved"
  fi
  echo "[$label] loading $resolved"
  docker load -i "$resolved"
}

sc_docker_resolve_release_svr_bin() {
  if [[ -n "$DOCKER_RELEASE_SVR_BIN" && -f "$DOCKER_RELEASE_SVR_BIN" ]]; then
    echo "$DOCKER_RELEASE_SVR_BIN"
    return 0
  fi
  local d="$DOCKER_RELEASE_SVR_DIR" b
  for b in "$d/starcrystalsvr" "$d/starcrystalsvr.exe"; do
    if [[ -f "$b" ]]; then
      echo "$b"
      return 0
    fi
  done
  for b in "$RELEASE_ROOT/starcrystalsvr" "$RELEASE_ROOT/starcrystalsvr.exe"; do
    if [[ -f "$b" ]]; then
      echo "$b"
      return 0
    fi
  done
  return 1
}

sc_docker_prepare_release_svr_dir() {
  local dir="$DOCKER_RELEASE_SVR_DIR" dest src
  mkdir -p "$dir"
  dest="$dir/starcrystalsvr"
  if [[ -f "$dest" ]]; then
    echo "$dir"
    return 0
  fi
  if [[ -n "$DOCKER_RELEASE_SVR_BIN" && -f "$DOCKER_RELEASE_SVR_BIN" ]]; then
    src="$DOCKER_RELEASE_SVR_BIN"
  elif src="$(sc_docker_resolve_release_svr_bin 2>/dev/null)" && [[ -n "$src" && -f "$src" && "$src" != "$dest" ]]; then
    :
  else
    echo "[docker_release] 未找到 starcrystalsvr，请编译后放到 $dest 或设置 DOCKER_RELEASE_SVR_BIN" >&2
    return 1
  fi
  cp -f "$src" "$dest"
  chmod +x "$dest" 2>/dev/null || true
  echo "[docker_release] 已放入 $dest（来自 $src）"
  echo "$dir"
}

sc_docker_release_apply_ignore() {
  local dst="$RELEASE_ROOT/.dockerignore"
  DOCKER_RELEASE_IGNORE_BACKUP=""
  if [[ -f "$dst" ]]; then
    DOCKER_RELEASE_IGNORE_BACKUP="$(mktemp "${TMPDIR:-/tmp}/sc-dockerignore.XXXXXX")"
    cp -f "$dst" "$DOCKER_RELEASE_IGNORE_BACKUP"
  fi
  cp -f "$DOCKER_RELEASE_CONTEXT/docker.releaseignore" "$dst"
}

sc_docker_release_restore_ignore() {
  local dst="$RELEASE_ROOT/.dockerignore"
  if [[ -n "${DOCKER_RELEASE_IGNORE_BACKUP:-}" && -f "$DOCKER_RELEASE_IGNORE_BACKUP" ]]; then
    cp -f "$DOCKER_RELEASE_IGNORE_BACKUP" "$dst"
    rm -f "$DOCKER_RELEASE_IGNORE_BACKUP"
  else
    rm -f "$dst"
  fi
  unset DOCKER_RELEASE_IGNORE_BACKUP
}

sc_docker_print_mirror_save_dir() {
  echo "镜像导出目录: $DOCKER_MIRROR_SAVE_DIR"
}

sc_docker_print_mysql_paths() {
  sc_docker_print_mirror_save_dir
  echo "镜像标签:     $DOCKER_MYSQL_IMAGE"
  echo "Dockerfile:   $DOCKER_MYSQL_CONTEXT/Dockerfile"
  echo "镜像内配置:   /etc/mysql/conf.d/starcrystal.cnf"
  echo "宿主数据目录: $DOCKER_MYSQL_DATA/  -> /var/lib/mysql"
  echo "宿主备份目录: $DOCKER_MYSQL_BACKUP_DIR/  -> /backups"
  echo "导出 tar:     $DOCKER_MYSQL_TAR"
  echo ""
  echo "查看本机镜像: docker images $DOCKER_MYSQL_IMAGE_REPO"
}

sc_docker_print_redis_paths() {
  sc_docker_print_mirror_save_dir
  echo "镜像标签:     $DOCKER_REDIS_IMAGE"
  echo "Dockerfile:   $DOCKER_REDIS_CONTEXT/Dockerfile"
  echo "镜像内配置:   /etc/redis/redis.conf"
  echo "宿主数据目录: $DOCKER_REDIS_DATA/  (dump.rdb / appendonly.aof)"
  echo "宿主备份目录: $DOCKER_REDIS_BACKUP_DIR/  -> /data/backups"
  echo "导出 tar:     $DOCKER_REDIS_TAR"
  echo ""
  echo "查看本机镜像: docker images $DOCKER_REDIS_IMAGE_REPO"
}

sc_docker_print_release_paths() {
  sc_docker_print_mirror_save_dir
  echo "镜像标签:     $DOCKER_RELEASE_IMAGE"
  echo "Dockerfile:   $DOCKER_RELEASE_CONTEXT/Dockerfile"
  echo "构建上下文:   $RELEASE_ROOT/（.dockerignore 排除 exe 与 log 内容）"
  echo "导出 tar:     $DOCKER_RELEASE_TAR"
  echo "运行时目录:   $DOCKER_RELEASE_SVR_DIR/ -> 容器 /app/starcrystalsvr/（内含 starcrystalsvr 二进制）"
  echo ""
  echo "查看本机镜像: docker images $DOCKER_RELEASE_IMAGE_REPO"
}
