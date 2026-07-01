#!/usr/bin/env bash
# 本地验收：模拟 /app/publish 布局测试 unpack.sh（不依赖真实压缩包内容）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
UNPACK="$SCRIPT_DIR/unpack.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PUBLISH_ROOT="$TMP/app/publish"
CDN_ROOT="$TMP/wwwroot/minigame.starlaneinfinite.com"
CDN_H5_DIR="$CDN_ROOT/h5"
CDN_H5_BACKUP_ROOT="$CDN_ROOT/_h5-predeploy-backup"
BUNDLE="$PUBLISH_ROOT/20260531-test001"
mkdir -p "$PUBLISH_ROOT" "$CDN_H5_DIR" "$CDN_H5_BACKUP_ROOT"

# 模拟已部署目录
for n in release dbscripts idip-webclient; do
  mkdir -p "$PUBLISH_ROOT/$n"
  echo "old-$n" >"$PUBLISH_ROOT/$n/marker.txt"
done

# 模拟 CDN h5 旧内容
mkdir -p "$CDN_H5_DIR/game-old"
echo old-h5 >"$CDN_H5_DIR/game-old/index.html"

# 最小 tar.gz（顶层 release/ dbscripts/ idip-webclient/ release_h5/）
STAGE="$TMP/stage"
mkdir -p "$STAGE/release" "$STAGE/dbscripts" "$STAGE/idip-webclient" "$STAGE/release_h5/game2"
echo new-release >"$STAGE/release/app.txt"
echo new-db >"$STAGE/dbscripts/app.txt"
echo new-idip >"$STAGE/idip-webclient/app.txt"
echo new-h5 >"$STAGE/release_h5/game2/index.html"

mkdir -p "$BUNDLE"
tar -czf "$BUNDLE/release.tar.gz" -C "$STAGE" release
tar -czf "$BUNDLE/dbscripts.tar.gz" -C "$STAGE" dbscripts
tar -czf "$BUNDLE/idip-webclient.tar.gz" -C "$STAGE" idip-webclient
tar -czf "$BUNDLE/release_h5.tar.gz" -C "$STAGE" release_h5
cp "$UNPACK" "$BUNDLE/unpack.sh"
chmod +x "$BUNDLE/unpack.sh"
sed 's/\r$//' "$BUNDLE/unpack.sh" >"$BUNDLE/unpack.sh.tmp" && mv "$BUNDLE/unpack.sh.tmp" "$BUNDLE/unpack.sh"

echo "==> dry-run"
(
  cd "$BUNDLE"
  UNPACK_DEPLOY_ROOT="$PUBLISH_ROOT" \
  UNPACK_CDN_H5_DIR="$CDN_H5_DIR" \
  UNPACK_CDN_H5_BACKUP_ROOT="$CDN_H5_BACKUP_ROOT" \
  bash ./unpack.sh --dry-run
)

echo "==> real unpack (with CDN h5)"
(
  cd "$BUNDLE"
  UNPACK_DEPLOY_ROOT="$PUBLISH_ROOT" \
  UNPACK_CDN_H5_DIR="$CDN_H5_DIR" \
  UNPACK_CDN_H5_BACKUP_ROOT="$CDN_H5_BACKUP_ROOT" \
  bash ./unpack.sh
)

for n in release dbscripts idip-webclient; do
  [[ -f "$PUBLISH_ROOT/$n/app.txt" ]] || { echo "FAIL missing $PUBLISH_ROOT/$n/app.txt"; exit 1; }
  grep -q "new-$n" "$PUBLISH_ROOT/$n/app.txt" 2>/dev/null || grep -q "new-" "$PUBLISH_ROOT/$n/app.txt"
  echo "OK deployed $n"
done

[[ -d "$PUBLISH_ROOT/_predeploy-backup/20260531-test001-"* ]] && echo "OK backup dir exists"
[[ ! -f "$PUBLISH_ROOT/release/marker.txt" ]] && echo "OK old release removed"

[[ -f "$CDN_H5_DIR/game2/index.html" ]] || { echo "FAIL missing CDN game2/index.html"; exit 1; }
grep -q new-h5 "$CDN_H5_DIR/game2/index.html"
[[ ! -d "$CDN_H5_DIR/game-old" ]] || { echo "FAIL old CDN h5 not cleared"; exit 1; }
echo "OK CDN h5 deployed"

backup_count="$(find "$CDN_H5_BACKUP_ROOT" -maxdepth 1 -name 'h5-*.tar.gz' 2>/dev/null | wc -l | tr -d ' ')"
[[ "$backup_count" -ge 1 ]] && echo "OK CDN h5 backup exists ($backup_count)"

echo "==> skip CDN h5 (UNPACK_SKIP_CDN_H5=1)"
echo skip-marker >"$CDN_H5_DIR/skip-marker.txt"
(
  cd "$BUNDLE"
  UNPACK_DEPLOY_ROOT="$PUBLISH_ROOT" \
  UNPACK_CDN_H5_DIR="$CDN_H5_DIR" \
  UNPACK_SKIP_CDN_H5=1 \
  bash ./unpack.sh
)
[[ -f "$CDN_H5_DIR/skip-marker.txt" ]] && echo "OK CDN h5 unchanged when skipped"

echo ""
echo "ACCEPT: unpack.sh passed"
