# 复制为 deploy-env.local.sh 后按需修改（deploy-idip-webclient.sh 会自动 source）
#
# cp deploy-env.local.example.sh deploy-env.local.sh

# StarCrystal API（Vitest / curl 直连，非浏览器 UI）
# 同机部署默认 127.0.0.1:8080；API 在其它机器时改为 http://192.168.x.x:8080
export IDIP_BASE_URL="${IDIP_BASE_URL:-http://127.0.0.1:8080}"

# 运营台对外 URL（Vitest webclient_deploy.test.ts）
# 留空则由脚本按 ENABLE_HTTPS + VERIFY_HOST 推导
# export IDIP_WEBCLIENT_URL="http://192.168.75.99"

# nginx 反代 upstream（通常与 API 同机）
export API_BACKEND_HOST="${API_BACKEND_HOST:-127.0.0.1}"
export API_BACKEND_PORT="${API_BACKEND_PORT:-8080}"

# 浏览器访问用主机名/IP（冒烟、Vitest）
export VERIFY_HOST="${VERIFY_HOST:-192.168.75.99}"
export CLOUD_PUBLIC_HOST="${CLOUD_PUBLIC_HOST:-$VERIFY_HOST}"

# 1=HTTPS(443+自签或已有证书)，0=仅 HTTP(80)
export ENABLE_HTTPS="${ENABLE_HTTPS:-0}"

# IDIP 鉴权（未设置时尝试从 release/configs/starcrystal.json 读取 idip.key / operators[0]）
export IDIP_KEY="${IDIP_KEY:-change-me-in-production}"
export IDIP_USERNAME="${IDIP_USERNAME:-holyjing}"
export IDIP_PASSWORD="${IDIP_PASSWORD:-jgyjgyjgy}"

# 首次 Linux 部署运营登录：在 server 根目录执行
#   chmod +x tools/idip-webclient/scripts/encrypt-idip-operator.sh
#   MERGE=1 tools/idip-webclient/scripts/encrypt-idip-operator.sh
# 脚本会随机生成 operatorCipherKey（Base64 混合字符）与复杂运营密码，并写入 starcrystal.json。
# 重新加密时须 export 相同的 IDIP_OPERATOR_CIPHER_KEY。
# export IDIP_OPERATOR_CIPHER_KEY=""

# 静态文件安装目录
export WEB_ROOT="${WEB_ROOT:-/var/www/starcrystal-idip/dist}"

# 构建：已有 dist/index.html 时默认跳过；FORCE_NPM_BUILD=1 强制重编
# export FORCE_NPM_BUILD=1
# export SKIP_REGRESSION=1
