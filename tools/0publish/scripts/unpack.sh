#!/usr/bin/env bash
# 外网部署解包：在发布子目录（含本脚本与四个压缩包）内执行。
#
# 部署根目录默认 /app/publish，其下应有 release、dbscripts、idip-webclient。
# 流程：备份并删除部署根下这三个目录 → 解压当前目录压缩包 → 移到部署根 →
#       可选 [4/4] 备份/清空/覆盖 CDN /h5（release_h5.tar.gz）。
#
# 用法:
#   cd /app/publish/yyyyMMdd-HHmmss && bash unpack.sh
#   UNPACK_DEPLOY_ROOT=/app/publish bash unpack.sh --dry-run
#   UNPACK_SKIP_CDN_H5=1 bash unpack.sh
set -euo pipefail

DEFAULT_DEPLOY_ROOT='/app/publish'
DEFAULT_CDN_H5_DIR='/wwwroot/minigame.starlaneinfinite.com/h5'
DEFAULT_CDN_H5_BACKUP_ROOT='/wwwroot/minigame.starlaneinfinite.com/_h5-predeploy-backup'
DEFAULT_CDN_H5_BACKUP_MAX=10
DRY_RUN=0

usage() {
  echo "Usage: $0 [--dry-run]" >&2
  echo "  Run inside publish bundle subdir (release/dbscripts/idip-webclient[/release_h5] archives + unpack.sh)." >&2
  echo "  Deploy root: UNPACK_DEPLOY_ROOT or ${DEFAULT_DEPLOY_ROOT} (default)." >&2
  echo "  CDN H5: UNPACK_CDN_H5_DIR, UNPACK_CDN_H5_BACKUP_ROOT, UNPACK_CDN_H5_BACKUP_MAX, UNPACK_SKIP_CDN_H5=1." >&2
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown option: $1" >&2; usage 1 ;;
  esac
  shift
done

PUBLISH_SUBDIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
NAMES=(release dbscripts idip-webclient)
CDN_H5_DIR="${UNPACK_CDN_H5_DIR:-$DEFAULT_CDN_H5_DIR}"
CDN_H5_BACKUP_ROOT="${UNPACK_CDN_H5_BACKUP_ROOT:-$DEFAULT_CDN_H5_BACKUP_ROOT}"
CDN_H5_BACKUP_MAX="${UNPACK_CDN_H5_BACKUP_MAX:-$DEFAULT_CDN_H5_BACKUP_MAX}"

resolve_deploy_root() {
  local root="${UNPACK_DEPLOY_ROOT:-${PUBLISH_DEPLOY_ROOT:-}}"
  if [[ -n "$root" ]]; then
    mkdir -p "$root"
    echo "$(cd "$root" && pwd)"
    return 0
  fi
  local parent
  parent="$(cd "$PUBLISH_SUBDIR/.." && pwd)"
  # 子目录在 /app/publish/yyyyMMdd-HHmmss 下时，父目录即部署根
  if [[ "$parent" == "$DEFAULT_DEPLOY_ROOT" ]]; then
    echo "$parent"
    return 0
  fi
  mkdir -p "$DEFAULT_DEPLOY_ROOT"
  echo "$(cd "$DEFAULT_DEPLOY_ROOT" && pwd)"
}

deploy_path() {
  local name="$1" root="$2"
  echo "$root/$name"
}

find_archive() {
  local name="$1" f
  for f in "$PUBLISH_SUBDIR/${name}.tar.gz" "$PUBLISH_SUBDIR/${name}.tgz" "$PUBLISH_SUBDIR/${name}.zip"; do
    [[ -f "$f" ]] && { echo "$f"; return 0; }
  done
  return 1
}

extract_archive() {
  local archive="$1" dest="$2"
  mkdir -p "$dest"
  case "$archive" in
    *.tar.gz|*.tgz) tar -xzf "$archive" -C "$dest" ;;
    *.zip)
      command -v unzip >/dev/null 2>&1 || { echo "unzip not found for $archive" >&2; return 1; }
      unzip -q -o "$archive" -d "$dest"
      ;;
    *)
      echo "Unsupported archive: $archive" >&2
      return 1
      ;;
  esac
}

run_or_echo() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

prune_cdn_h5_backups() {
  local root="$1" max="$2"
  [[ -d "$root" ]] || return 0
  local files=() f
  shopt -s nullglob
  for f in "$root"/h5-*.tar.gz; do
    files+=("$f")
  done
  shopt -u nullglob
  ((${#files[@]} <= max)) && return 0
  local sorted=() i
  mapfile -t sorted < <(ls -1t "${files[@]}")
  for (( i=max; i<${#sorted[@]}; i++ )); do
    echo "    prune old backup: ${sorted[$i]}"
    rm -f "${sorted[$i]}"
  done
}

sync_release_h5_to_cdn() {
  local src="$1" dest="$2"
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --delete "${src}/" "${dest}/"
  else
    mkdir -p "$dest"
    find "$dest" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
    cp -a "${src}/." "$dest/"
  fi
}

deploy_cdn_h5() {
  local archive staging backup_file h5_parent h5_base ts
  archive="$(find_archive release_h5)" || {
    echo "WARN: no release_h5 archive in bundle; skip CDN H5 deploy" >&2
    return 0
  }

  h5_parent="$(cd "$(dirname "$CDN_H5_DIR")" && pwd)"
  h5_base="$(basename "$CDN_H5_DIR")"
  ts="$(date +%Y%m%d-%H%M%S)"
  backup_file="$CDN_H5_BACKUP_ROOT/h5-$(basename "$PUBLISH_SUBDIR")-${ts}.tar.gz"
  staging="$PUBLISH_SUBDIR/_unpack_cdn_staging"

  echo ""
  echo "==> [4/4] CDN H5 deploy"
  echo "    cdn h5 dir:     $CDN_H5_DIR"
  echo "    backup root:    $CDN_H5_BACKUP_ROOT"
  echo "    backup file:    $backup_file"
  echo "    archive:        $(basename "$archive")"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "[dry-run] mkdir -p $CDN_H5_DIR"
    echo "[dry-run] backup $CDN_H5_DIR -> $backup_file"
    echo "[dry-run] prune $CDN_H5_BACKUP_ROOT (keep $CDN_H5_BACKUP_MAX)"
    echo "[dry-run] clear $CDN_H5_DIR/*"
    echo "[dry-run] extract $(basename "$archive") -> staging/release_h5/"
    echo "[dry-run] sync staging/release_h5/ -> $CDN_H5_DIR/"
    return 0
  fi

  mkdir -p "$CDN_H5_DIR" "$CDN_H5_BACKUP_ROOT"
  if ! [[ -w "$CDN_H5_DIR" ]]; then
    echo "CDN_H5_DIR not writable: $CDN_H5_DIR" >&2
    exit 1
  fi

  if [[ -d "$CDN_H5_DIR" ]] && [[ -n "$(ls -A "$CDN_H5_DIR" 2>/dev/null || true)" ]]; then
    echo "    backup $CDN_H5_DIR -> $backup_file"
    tar -czf "$backup_file" -C "$h5_parent" "$h5_base"
    prune_cdn_h5_backups "$CDN_H5_BACKUP_ROOT" "$CDN_H5_BACKUP_MAX"
  else
    echo "    skip backup (cdn h5 empty or missing content)"
  fi

  echo "    clear $CDN_H5_DIR"
  find "$CDN_H5_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +

  rm -rf "$staging"
  mkdir -p "$staging"
  echo "    extract $(basename "$archive")"
  extract_archive "$archive" "$staging"
  [[ -d "$staging/release_h5" ]] || {
    echo "Archive did not contain top-level release_h5/: $archive" >&2
    rm -rf "$staging"
    exit 1
  }

  echo "    sync release_h5 -> $CDN_H5_DIR"
  sync_release_h5_to_cdn "$staging/release_h5" "$CDN_H5_DIR"
  rm -rf "$staging"
}

DEPLOY_ROOT="$(resolve_deploy_root)"
BACKUP_DIR="$DEPLOY_ROOT/_predeploy-backup/$(basename "$PUBLISH_SUBDIR")-$(date +%Y%m%d-%H%M%S)"
STAGING="$PUBLISH_SUBDIR/_unpack_staging"

echo "==> StarCrystal unpack (external publish)"
echo "    bundle subdir:  $PUBLISH_SUBDIR"
echo "    deploy root:    $DEPLOY_ROOT"
echo "    backup dir:     $BACKUP_DIR"
[[ "$DRY_RUN" -eq 1 ]] && echo "    (dry-run)"

for name in "${NAMES[@]}"; do
  archive="$(find_archive "$name")" || {
    echo "Missing archive in bundle subdir: ${name}.tar.gz or ${name}.zip" >&2
    exit 1
  }
  echo "    archive: $(basename "$archive")"
done

echo ""
echo "==> [1/4] backup /app/publish dirs (release, dbscripts, idip-webclient) then remove"
for name in "${NAMES[@]}"; do
  target="$(deploy_path "$name" "$DEPLOY_ROOT")"
  if [[ ! -e "$target" ]]; then
    echo "    skip $name (not present: $target)"
    continue
  fi
  run_or_echo mkdir -p "$BACKUP_DIR"
  backup_file="$BACKUP_DIR/${name}.tar.gz"
  echo "    backup $target -> $backup_file"
  if [[ "$DRY_RUN" -eq 0 ]]; then
    tar -czf "$backup_file" -C "$DEPLOY_ROOT" "$name"
    chmod -R u+w "$target" 2>/dev/null || true
    rm -rf "$target"
  fi
done

echo ""
echo "==> [2/4] extract archives in bundle subdir"
if [[ "$DRY_RUN" -eq 0 ]]; then
  rm -rf "$STAGING"
  mkdir -p "$STAGING"
fi
for name in "${NAMES[@]}"; do
  archive="$(find_archive "$name")"
  echo "    extract $(basename "$archive")"
  if [[ "$DRY_RUN" -eq 0 ]]; then
    extract_archive "$archive" "$STAGING"
    [[ -d "$STAGING/$name" ]] || {
      echo "Archive did not contain top-level ${name}/ : $archive" >&2
      exit 1
    }
  fi
done

echo ""
echo "==> [3/4] move extracted dirs to $DEPLOY_ROOT"
for name in "${NAMES[@]}"; do
  target="$(deploy_path "$name" "$DEPLOY_ROOT")"
  echo "    $name -> $target"
  if [[ "$DRY_RUN" -eq 0 ]]; then
    mkdir -p "$DEPLOY_ROOT"
    mv "$STAGING/$name" "$target"
  fi
done

if [[ "$DRY_RUN" -eq 0 ]]; then
  rm -rf "$STAGING"
fi

if [[ "${UNPACK_SKIP_CDN_H5:-}" == "1" ]]; then
  echo ""
  echo "==> [4/4] skip CDN H5 (UNPACK_SKIP_CDN_H5=1)"
else
  deploy_cdn_h5
fi

echo ""
echo "[unpack] done."
echo "  backups: $BACKUP_DIR"
echo "  deployed: $DEPLOY_ROOT/{release,dbscripts,idip-webclient}"
if [[ "${UNPACK_SKIP_CDN_H5:-}" != "1" ]] && find_archive release_h5 >/dev/null 2>&1; then
  echo "  cdn h5:   $CDN_H5_DIR"
fi
