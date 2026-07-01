#!/usr/bin/env bash
# 校验 pack-publish 输出的四个 tar.gz（结构 + 可选二进制 smoke）
# 用法:
#   bash tools/0publish/scripts/pack-publish-verify.sh --dir tools/0publish/20260531-120000
#   bash tools/0publish/scripts/pack-publish-verify.sh --latest
#   bash tools/0publish/scripts/pack-publish-verify.sh --latest --smoke
set -euo pipefail

VERIFY_DIR=""
USE_LATEST=0
SMOKE=0

usage() {
  echo "Usage: $0 --dir <publish-output-dir> | --latest [--smoke] [--repo-root path]" >&2
  exit 1
}

REPO_OVERRIDE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)
      shift
      [[ $# -gt 0 ]] || usage
      VERIFY_DIR="$1"
      ;;
    --latest) USE_LATEST=1 ;;
    --smoke) SMOKE=1 ;;
    --repo-root)
      shift
      [[ $# -gt 0 ]] || usage
      REPO_OVERRIDE="$1"
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
    _c="$_Y_SVR_ROOT/tools/0publish/scripts/pack-publish-verify.sh"
    [[ -f "$_c" ]] && exec bash "$_c" "$@"
    echo "Run from .../tools/0publish/scripts (Y: or $_LINUX_SVR_ROOT)" >&2
    exit 1
    ;;
esac
# shellcheck source=../../scripts/starcrystal-server-root.sh
source "$_SCRIPT_DIR/../../scripts/starcrystal-server-root.sh"
REPO_ROOT="$(sc_server_root "$REPO_OVERRIDE")"
sc_assert_publish_root "$REPO_ROOT" || exit 1
PUBLISH_ROOT="$REPO_ROOT/tools/0publish"

if [[ "$USE_LATEST" -eq 1 ]]; then
  VERIFY_DIR=""
  for d in $(ls -1t "$PUBLISH_ROOT" 2>/dev/null); do
    [[ "$d" =~ ^[0-9]{8}-[0-9]{6}$ ]] || continue
    [[ -f "$PUBLISH_ROOT/$d/release.tar.gz" || -f "$PUBLISH_ROOT/$d/release.zip" ]] || continue
    [[ -f "$PUBLISH_ROOT/$d/release_h5.tar.gz" || -f "$PUBLISH_ROOT/$d/release_h5.zip" ]] || continue
    VERIFY_DIR="$PUBLISH_ROOT/$d"
    break
  done
  [[ -n "$VERIFY_DIR" ]] || { echo "No complete yyyyMMdd-HHmmss publish under $PUBLISH_ROOT" >&2; exit 1; }
fi

[[ -n "$VERIFY_DIR" ]] || usage
[[ -d "$VERIFY_DIR" ]] || { echo "Not a directory: $VERIFY_DIR" >&2; exit 1; }
VERIFY_DIR="$(cd "$VERIFY_DIR" && pwd)"
PUBLISH_TAG="$(basename "$VERIFY_DIR")"
PUBLISH_TAG="${PUBLISH_TAG//$'\r'/}"
BUNDLE_TAR="$PUBLISH_ROOT/${PUBLISH_TAG}.tar.gz"
BUNDLE_ZIP="$PUBLISH_ROOT/${PUBLISH_TAG}.zip"

pass=0
fail=0

ok() { echo "[PASS] $*"; pass=$((pass + 1)); }
bad() { echo "[FAIL] $*" >&2; fail=$((fail + 1)); }

resolve_archive() {
  local name="$1" f
  for f in "$VERIFY_DIR/${name}.tar.gz" "$VERIFY_DIR/${name}.tgz" "$VERIFY_DIR/${name}.zip"; do
    [[ -f "$f" ]] && { echo "$f"; return 0; }
  done
  return 1
}

tar_list() {
  tar -tzf "$1" 2>/dev/null | tr -d '\r'
}

zip_list() {
  unzip -l "$1" 2>/dev/null | awk 'NR>3 && NF>=4 {print $4}' | tr '\\' '/' | tr -d '\r'
}

archive_list() {
  case "$1" in
    *.zip) zip_list "$1" ;;
    *) tar_list "$1" ;;
  esac
}

check_archive() {
  local name="$1" top="$2" file list
  file="$(resolve_archive "$name")" || {
    bad "missing ${name}.tar.gz or ${name}.zip"
    return
  }
  list="$(archive_list "$file")" || { bad "invalid archive: $file"; return; }
  if ! grep -qE "^${top}/" <<<"$list"; then
    bad "$file: expected top-level ${top}/"
    return
  fi
  ok "$(basename "$file") -> ${top}/"
}

echo "==> pack-publish verify"
echo "    dir: $VERIFY_DIR"

check_archive "release" "release"
check_archive "dbscripts" "dbscripts"
check_archive "idip-webclient" "idip-webclient"
check_archive "release_h5" "release_h5"

[[ -f "$VERIFY_DIR/pack-manifest.txt" ]] && ok "pack-manifest.txt" || bad "missing pack-manifest.txt"
[[ -f "$VERIFY_DIR/unpack.sh" ]] && ok "unpack.sh" || bad "missing unpack.sh"

bundle_file=""
bundle_list=""
if [[ -f "$BUNDLE_TAR" ]]; then
  bundle_file="$BUNDLE_TAR"
  bundle_list="$(tar_list "$BUNDLE_TAR")" || bundle_list=""
elif [[ -f "$BUNDLE_ZIP" ]]; then
  bundle_file="$BUNDLE_ZIP"
  bundle_list="$(zip_list "$BUNDLE_ZIP")" || bundle_list=""
fi

if [[ -n "$bundle_file" ]]; then
  if grep -qE "^${PUBLISH_TAG}/release\.(tar\.gz|zip)$" <<<"$bundle_list"; then
    ok "$(basename "$bundle_file") contains ${PUBLISH_TAG}/release archive"
  else
    bad "$(basename "$bundle_file") missing ${PUBLISH_TAG}/release archive"
  fi
  if grep -qE "^${PUBLISH_TAG}/dbscripts\.(tar\.gz|zip)$" <<<"$bundle_list"; then
    ok "$(basename "$bundle_file") contains ${PUBLISH_TAG}/dbscripts archive"
  else
    bad "$(basename "$bundle_file") missing ${PUBLISH_TAG}/dbscripts archive"
  fi
  if grep -qE "^${PUBLISH_TAG}/idip-webclient\.(tar\.gz|zip)$" <<<"$bundle_list"; then
    ok "$(basename "$bundle_file") contains ${PUBLISH_TAG}/idip-webclient archive"
  else
    bad "$(basename "$bundle_file") missing ${PUBLISH_TAG}/idip-webclient archive"
  fi
  if grep -qE "^${PUBLISH_TAG}/release_h5\.(tar\.gz|zip)$" <<<"$bundle_list"; then
    ok "$(basename "$bundle_file") contains ${PUBLISH_TAG}/release_h5 archive"
  else
    bad "$(basename "$bundle_file") missing ${PUBLISH_TAG}/release_h5 archive"
  fi
  if grep -qE "^${PUBLISH_TAG}/unpack\.sh$" <<<"$bundle_list"; then
    ok "$(basename "$bundle_file") contains ${PUBLISH_TAG}/unpack.sh"
  else
    bad "$(basename "$bundle_file") missing ${PUBLISH_TAG}/unpack.sh"
  fi
else
  bad "missing bundle ${PUBLISH_TAG}.tar.gz or ${PUBLISH_TAG}.zip"
fi

ARCH_RELEASE="$(resolve_archive release 2>/dev/null || true)"
if [[ -n "$ARCH_RELEASE" ]]; then
  rel_list="$(archive_list "$ARCH_RELEASE")"
  if grep -qx 'release/starcrystalsvr' <<<"$rel_list"; then
    ok "release/starcrystalsvr in archive"
  else
    bad "release/starcrystalsvr not in archive"
  fi
  if grep -q 'release/configs/starcrystal.json' <<<"$rel_list"; then
    ok "release/configs/starcrystal.json in archive"
  else
    bad "release/configs/starcrystal.json not in archive"
  fi
fi

if [[ "$SMOKE" -eq 1 && -n "${ARCH_RELEASE:-}" ]]; then
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  case "$ARCH_RELEASE" in
    *.zip) unzip -q -o "$ARCH_RELEASE" -d "$tmp" ;;
    *) tar -xzf "$ARCH_RELEASE" -C "$tmp" ;;
  esac
  bin="$tmp/release/starcrystalsvr"
  if [[ -x "$bin" ]]; then
    if "$bin" --help >/dev/null 2>&1 || "$bin" -h >/dev/null 2>&1 || true; then
      ok "starcrystalsvr runs (--help/-h or no-op)"
    else
      # 无 --help 时仅检查可执行
      file "$bin" | grep -qi 'elf' && ok "starcrystalsvr ELF executable" || bad "starcrystalsvr not ELF"
    fi
  else
    bad "starcrystalsvr not executable after extract"
  fi
fi

echo ""
echo "Result: $pass passed, $fail failed"
[[ "$fail" -eq 0 ]]
