#!/usr/bin/env bash
# StarCrystal 发布打包（Linux）
#   tools/0publish/yyyyMMdd-HHmmss/  → release|dbscripts|idip-webclient|release_h5 四个 .tar.gz
#   tools/0publish/yyyyMMdd-HHmmss.tar.gz → 子目录总包
# 用法:
#   cd /home/holyjing/starcrystalsvr && bash tools/0publish/scripts/pack-publish.sh
#   bash tools/0publish/scripts/pack-publish.sh --skip-build
set -euo pipefail

SKIP_BUILD=0
BUILD_IDIP=0
PUBLISH_DIR=""
REPO_ROOT_OVERRIDE=""

usage() {
  echo "Usage: $0 [--skip-build] [--build-idip] [--publish-dir yyyyMMdd-HHmmss] [--repo-root path]" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build) SKIP_BUILD=1 ;;
    --build-idip) BUILD_IDIP=1 ;;
    --publish-dir|--date)
      shift
      [[ $# -gt 0 ]] || usage
      PUBLISH_DIR="$1"
      ;;
    --repo-root)
      shift
      [[ $# -gt 0 ]] || usage
      REPO_ROOT_OVERRIDE="$1"
      ;;
    -h|--help) usage ;;
    *) echo "Unknown option: $1" >&2; usage ;;
  esac
  shift
done

_Y_SVR_ROOT='Y:/holyjing/starcrystalsvr'
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
_LINUX_SVR_ROOT='/home/holyjing/starcrystalsvr'
case "$_SCRIPT_DIR" in
  [Yy]:/holyjing/starcrystalsvr/tools/0publish/scripts|[Yy]:\\holyjing\\starcrystalsvr\\tools\\0publish\\scripts) ;;
  /[yY]/holyjing/starcrystalsvr/tools/0publish/scripts) ;;
  "$_LINUX_SVR_ROOT/tools/0publish/scripts") ;;
  *)
    _canonical="$_Y_SVR_ROOT/tools/0publish/scripts/pack-publish.sh"
    if [[ -f "$_canonical" ]]; then
      echo "==> delegate to $_canonical"
      exec bash "$_canonical" "$@"
    fi
    echo "pack-publish.sh must run under .../tools/0publish/scripts (Y: or $_LINUX_SVR_ROOT)" >&2
    exit 1
    ;;
esac
# shellcheck source=../../scripts/starcrystal-server-root.sh
source "$_SCRIPT_DIR/../../scripts/starcrystal-server-root.sh"
REPO_ROOT="$(sc_server_root "$REPO_ROOT_OVERRIDE")"
sc_assert_publish_root "$REPO_ROOT" || exit 1
PUBLISH_ROOT="$REPO_ROOT/tools/0publish"
mkdir -p "$PUBLISH_ROOT"
TOOLS_ROOT="$REPO_ROOT/tools"
RELEASE_SRC="$REPO_ROOT/release"
RELEASE_H5_SRC="$REPO_ROOT/release_h5"
DBSCRIPTS_SRC="$TOOLS_ROOT/scripts/dbscripts"
IDIP_SRC="$TOOLS_ROOT/idip-webclient"
BUILD_SCRIPT="$TOOLS_ROOT/scripts/build.sh"

if [[ -z "$PUBLISH_DIR" ]]; then
  PUBLISH_DIR="$(date +%Y%m%d-%H%M%S)"
fi
if [[ ! "$PUBLISH_DIR" =~ ^[0-9]{8}(-[0-9]{6})?$ ]]; then
  echo "Invalid --publish-dir '$PUBLISH_DIR'; use yyyyMMdd-HHmmss" >&2
  exit 1
fi

OUTPUT_DIR="$PUBLISH_ROOT/$PUBLISH_DIR"
STAGING_ROOT="$OUTPUT_DIR/_staging"
TAR_RELEASE="$OUTPUT_DIR/release.tar.gz"
TAR_DBSCRIPTS="$OUTPUT_DIR/dbscripts.tar.gz"
TAR_IDIP="$OUTPUT_DIR/idip-webclient.tar.gz"
TAR_RELEASE_H5="$OUTPUT_DIR/release_h5.tar.gz"
MANIFEST_PATH="$OUTPUT_DIR/pack-manifest.txt"
BUNDLE_TAR="$PUBLISH_ROOT/$PUBLISH_DIR.tar.gz"

rm_tree() {
  local p="$1"
  [[ -e "$p" ]] || return 0
  chmod -R u+w "$p" 2>/dev/null || true
  rm -rf "$p"
}

copy_tree() {
  local src="$1" dest="$2"
  shift 2
  [[ -d "$src" ]] || { echo "Source not found: $src" >&2; return 1; }
  mkdir -p "$dest"
  if command -v rsync >/dev/null 2>&1; then
    local -a args=(-a --delete)
    local pat
    for pat in "$@"; do
      [[ -n "$pat" ]] && args+=(--exclude="$pat")
    done
    rsync "${args[@]}" "$src/" "$dest/"
    return 0
  fi
  find "$dest" -mindepth 1 -delete 2>/dev/null || rm -rf "${dest:?}"/* 2>/dev/null || true
  mkdir -p "$dest"
  cp -a "$src/." "$dest/" 2>/dev/null || cp -r "$src/." "$dest/"
}

copy_release() {
  local dest="$1"
  copy_tree "$RELEASE_SRC" "$dest" "*.pid" ".dockerignore"
  mkdir -p "$dest/log"
  find "$dest/log" -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true
  touch "$dest/log/.gitkeep"
  if [[ -f "$dest/starcrystalsvr.exe" ]]; then
    rm -f "$dest/starcrystalsvr.exe"
  fi
  if [[ ! -f "$dest/starcrystalsvr" ]]; then
    echo "WARN: missing release/starcrystalsvr (run build or copy Linux binary)" >&2
  else
    chmod +x "$dest/starcrystalsvr" 2>/dev/null || true
  fi
}

copy_dbscripts() {
  local dest="$1"
  copy_tree "$DBSCRIPTS_SRC" "$dest" \
    "data/mysql/" "data/mysql_backup/" "redis/linux/" \
    "*.pid"
  mkdir -p "$dest/data/mysql" "$dest/data/mysql_backup"
}

copy_idip() {
  local dest="$1"
  copy_tree "$IDIP_SRC" "$dest" \
    "node_modules/" ".tmp-regression/" "tmp-regression/" \
    "scripts_encrypt/" \
    "*.local" "tsconfig.tsbuildinfo"
}

copy_release_h5() {
  local dest="$1"
  if [[ ! -d "$RELEASE_H5_SRC" ]]; then
    echo "WARN: release_h5 not found: $RELEASE_H5_SRC (packing empty tree)" >&2
    mkdir -p "$dest"
    return 0
  fi
  copy_tree "$RELEASE_H5_SRC" "$dest" ".upload-*"
}

write_tar() {
  local name="$1" out="$2"
  rm -f "$out"
  tar -czf "$out" -C "$STAGING_ROOT" "$name"
  ls -lh "$out"
}

write_manifest() {
  cat >"$MANIFEST_PATH" <<EOF
StarCrystal pack-publish (linux)
generated: $(date '+%Y-%m-%d %H:%M:%S')
publishDir: $PUBLISH_DIR
repoRoot: $REPO_ROOT
outputDir: $OUTPUT_DIR
platform: $(uname -srm 2>/dev/null || uname -a)

archives (in publish subdir):
  release.tar.gz        (release/ API runtime)
  dbscripts.tar.gz      (dbscripts/ DB ops)
  idip-webclient.tar.gz (idip-webclient/ IDIP UI)
  release_h5.tar.gz     (release_h5/ H5 CDN payload)
  unpack.sh             (deploy: run inside this subdir)

bundle (tools/0publish/): $BUNDLE_TAR
EOF
}

echo "==> StarCrystal pack-publish ($PUBLISH_DIR)"
echo "    repo: $REPO_ROOT"
echo "    out:  $OUTPUT_DIR"

if [[ "$SKIP_BUILD" -eq 0 ]]; then
  [[ -f "$BUILD_SCRIPT" ]] || { echo "Build script not found: $BUILD_SCRIPT" >&2; exit 1; }
  echo "==> build (tools/scripts/build.sh)"
  bash "$BUILD_SCRIPT"
else
  echo "==> skip build (--skip-build)"
fi

if [[ "$BUILD_IDIP" -eq 1 ]]; then
  [[ -f "$IDIP_SRC/package.json" ]] || { echo "idip-webclient not found: $IDIP_SRC" >&2; exit 1; }
  echo "==> idip-webclient npm run build"
  (
    cd "$IDIP_SRC"
    if [[ ! -d node_modules ]]; then
      npm ci 2>/dev/null || npm install
    fi
    npm run build
  )
fi

[[ -d "$RELEASE_SRC" ]] || { echo "release/ not found: $RELEASE_SRC" >&2; exit 1; }
[[ -d "$DBSCRIPTS_SRC" ]] || { echo "dbscripts not found: $DBSCRIPTS_SRC" >&2; exit 1; }
[[ -d "$IDIP_SRC" ]] || { echo "idip-webclient not found: $IDIP_SRC" >&2; exit 1; }

mkdir -p "$OUTPUT_DIR"
echo "==> stage -> $STAGING_ROOT"
rm_tree "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT/release" "$STAGING_ROOT/dbscripts" "$STAGING_ROOT/idip-webclient" "$STAGING_ROOT/release_h5"

echo "==> pack release"
copy_release "$STAGING_ROOT/release"
write_tar "release" "$TAR_RELEASE"

echo "==> pack dbscripts"
copy_dbscripts "$STAGING_ROOT/dbscripts"
write_tar "dbscripts" "$TAR_DBSCRIPTS"

echo "==> pack idip-webclient"
copy_idip "$STAGING_ROOT/idip-webclient"
write_tar "idip-webclient" "$TAR_IDIP"

echo "==> pack release_h5"
copy_release_h5 "$STAGING_ROOT/release_h5"
write_tar "release_h5" "$TAR_RELEASE_H5"

rm_tree "$STAGING_ROOT"
write_manifest

copy_unpack_sh() {
  local src="$1" dest="$2"
  [[ -f "$src" ]] || return 1
  sed 's/\r$//' "$src" >"$dest"
  chmod +x "$dest" 2>/dev/null || true
}

UNPACK_SRC="$_SCRIPT_DIR/unpack.sh"
if copy_unpack_sh "$UNPACK_SRC" "$OUTPUT_DIR/unpack.sh"; then
  echo "==> copy unpack.sh (LF) -> $OUTPUT_DIR/unpack.sh"
else
  echo "WARN: missing $UNPACK_SRC" >&2
fi

echo "==> pack bundle (publish subdir -> 0publish root)"
rm -f "$BUNDLE_TAR"
tar -czf "$BUNDLE_TAR" -C "$PUBLISH_ROOT" "$PUBLISH_DIR"
ls -lh "$BUNDLE_TAR"

echo ""
echo "Done."
echo "  subdir:  $OUTPUT_DIR"
echo "    $TAR_RELEASE"
echo "    $TAR_DBSCRIPTS"
echo "    $TAR_IDIP"
echo "    $TAR_RELEASE_H5"
echo "    $MANIFEST_PATH"
echo "  bundle: $BUNDLE_TAR"
echo ""
echo "Verify: bash tools/0publish/scripts/pack-publish-verify.sh --dir $OUTPUT_DIR"
