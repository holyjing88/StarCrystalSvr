StarCrystal Nginx 反向代理

模板:
  tools/nginx/starcrystal-http.conf   仅 HTTP :80
  tools/nginx/starcrystal-https.conf  HTTP→HTTPS + :443

安装 (需 root):
  bash tools/scripts/nginx/install-starcrystal-nginx.sh
  ENABLE_HTTPS=1 bash tools/scripts/nginx/install-starcrystal-nginx.sh

当前 Linux 已执行 ENABLE_HTTPS=1：
  https://192.168.75.99/  → 127.0.0.1:8080
  http://192.168.75.99/   → 301 跳转 HTTPS
  自签名证书: /etc/nginx/ssl/starcrystal.crt
  release/configs/starcrystal.json useHttps=true

注意: useHttps=true 后勿直连 :8080（会 403），请走 Nginx 443。
