@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo 工作目录: %CD%
echo 启动本地 HTTP 服务（端口 8765），在浏览器打开: http://127.0.0.1:8765
echo 此窗口需保持运行，关闭后 http://127.0.0.1:8765 将无法打开
echo 按 Ctrl+C 可停止服务
netstat -an | findstr ":8765 " | findstr "LISTENING" >nul
if %ERRORLEVEL%==0 (
  echo 警告: 8765 端口可能已被占用, 如页面异常请换端口或结束占用进程
)
echo.
where py >nul 2>&1
if %ERRORLEVEL%==0 (
  py -3 -m http.server 8765
  goto :end
)
where python >nul 2>&1
if %ERRORLEVEL%==0 (
  python -m http.server 8765
  goto :end
)
where node >nul 2>&1
if %ERRORLEVEL%==0 (
  npx --yes serve -l 8765
  goto :end
)
echo 未找到 py / python / node。请安装 Python 3 或 Node.js，或在本目录用 Cocos/VS 的 "Live Server" 打开。
pause
:end
