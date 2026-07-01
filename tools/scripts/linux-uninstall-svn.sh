#!/bin/bash
# 卸载 Linux 本机 SVN 客户端（代码仅在 Windows 工作副本 svn update/commit，Linux 通过 SMB 共享目录读代码）。
set -euo pipefail

if command -v svn >/dev/null 2>&1; then
  echo "before: $(svn --version -q 2>/dev/null || svn --version | head -1)"
else
  echo "svn not installed"
  exit 0
fi

if command -v yum >/dev/null 2>&1; then
  sudo yum remove -y subversion
elif command -v apt-get >/dev/null 2>&1; then
  sudo apt-get remove -y subversion
else
  echo "unsupported package manager; remove subversion manually"
  exit 1
fi

command -v svn >/dev/null 2>&1 && { echo "FAIL: svn still present"; exit 1; }
echo "OK: subversion removed"
