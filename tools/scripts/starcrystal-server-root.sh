#!/usr/bin/env bash
# shellcheck disable=SC2034
# 服务器仓库根目录。Windows/SVN: Y:/holyjing/starcrystalsvr；Linux: /home/holyjing/starcrystalsvr
# 可用 STARCrystalSVR_ROOT 覆盖（勿指向 d:/0_games 开发目录）。

sc_server_root_default() {
  case "$(uname -s 2>/dev/null || echo unknown)" in
    Linux|Darwin)
      echo '/home/holyjing/starcrystalsvr'
      ;;
    *)
      echo 'Y:/holyjing/starcrystalsvr'
      ;;
  esac
}

sc_server_root() {
  local override="${1:-}"
  local root
  if [[ -n "$override" ]]; then
    root="$(cd "$override" && pwd)"
    echo "$root"
    return 0
  fi
  if [[ -n "${STARCrystalSVR_ROOT:-}" ]]; then
    root="$(cd "$STARCrystalSVR_ROOT" && pwd)"
    echo "$root"
    return 0
  fi
  root="$(sc_server_root_default)"
  if [[ ! -d "$root" ]]; then
    echo "starcrystal-server-root: server root not found: $root" >&2
    echo "  Windows SVN: Y:/holyjing/starcrystalsvr" >&2
    echo "  Linux: /home/holyjing/starcrystalsvr or STARCrystalSVR_ROOT" >&2
    return 1
  fi
  echo "$(cd "$root" && pwd)"
}

sc_assert_windows_y_root() {
  local root="$1"
  case "$(uname -s 2>/dev/null)" in
    MINGW*|MSYS*|CYGWIN*)
      if [[ "$root" != [Yy]:/holyjing/starcrystalsvr* && "$root" != [Yy]:\\holyjing\\starcrystalsvr* \
        && "$root" != /[yY]/holyjing/starcrystalsvr* ]]; then
        echo "pack-publish: Windows 发布根目录必须是 Y:/holyjing/starcrystalsvr（当前: $root）" >&2
        return 1
      fi
      ;;
  esac
  return 0
}

# pack-publish 用：Windows 校验 Y:，Linux 校验 /home/holyjing/starcrystalsvr
sc_assert_publish_root() {
  local root="$1"
  case "$(uname -s 2>/dev/null)" in
    Linux|Darwin)
      if [[ "$root" != /home/holyjing/starcrystalsvr* ]]; then
        echo "pack-publish: Linux 发布根目录必须是 /home/holyjing/starcrystalsvr（当前: $root）" >&2
        return 1
      fi
      ;;
    *)
      sc_assert_windows_y_root "$root" || return 1
      ;;
  esac
  return 0
}
