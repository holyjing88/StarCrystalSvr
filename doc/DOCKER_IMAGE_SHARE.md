# 将本机 Docker 镜像分发到其他服务器

| 组件 | 镜像标签 | 导出 tar（统一目录） |
|------|----------|----------------------|
| MySQL | `starcrystal/mysql:8.4` | `tools/docker/mirror_save/mysql/starcrystal-mysql-8.4.tar` |
| Redis | `starcrystal/redis:7-alpine` | `tools/docker/mirror_save/redis/starcrystal-redis-7-alpine.tar` |
| Release | `starcrystal/release:bundle` | `tools/docker/mirror_save/release/starcrystal-release-bundle.tar` |

脚本：`docker_mysql.sh` / `docker_redis.sh` / `docker_release.sh` 的 `build` | `save` | `load` | `start`。  
一键导出三台：`bash tools/docker/docker_mirror_save.sh`

Release 镜像**不含** `starcrystalsvr.exe`（及 Linux 二进制）；`log/` 仅空目录。目标机需自备 `starcrystalsvr` 并挂载到 `/app/starcrystalsvr`。

## 方式一：离线 tar 包（推荐内网 / 无仓库）

### A 机（构建并导出）

```bash
cd server-go
bash tools/docker/docker_mirror_save.sh
```

或分步：

```bash
bash tools/docker/docker_mysql.sh build && bash tools/docker/docker_mysql.sh save
bash tools/docker/docker_redis.sh build && bash tools/docker/docker_redis.sh save
bash tools/docker/docker_release.sh build && bash tools/docker/docker_release.sh save
```

拷到 B 机：

```bash
scp -r tools/docker/mirror_save user@B:/opt/starcrystal/server-go/tools/docker/
scp -r tools/docker/*.sh tools/docker/lib user@B:/opt/starcrystal/server-go/tools/docker/
scp -r tools/scripts user@B:/opt/starcrystal/server-go/tools/
```

### B 机（导入并启动）

```bash
cd /opt/starcrystal/server-go
bash tools/docker/docker_mysql.sh load
bash tools/docker/docker_redis.sh load
bash tools/docker/docker_release.sh load
bash tools/docker/docker_startdb.sh
# 将 Linux 版 starcrystalsvr 放到某路径后：
export DOCKER_RELEASE_SVR_BIN=/opt/starcrystal/starcrystalsvr
bash tools/docker/docker_release.sh start
```

仅 tar、无脚本时：`docker load -i mirror_save/mysql/...` 等。

验证：

```bash
docker ps
curl -s http://127.0.0.1:8080/health || true
docker exec -it starcrystal-dev-mysql mysql -uroot -proot -e "SHOW DATABASES;"
docker exec -it starcrystal-dev-redis redis-cli ping
```

---

## 方式二：私有镜像仓库

```bash
docker tag starcrystal/mysql:8.4 registry.example.com/starcrystal/mysql:8.4
docker tag starcrystal/redis:7-alpine registry.example.com/starcrystal/redis:7-alpine
docker tag starcrystal/release:bundle registry.example.com/starcrystal/release:bundle
docker push ...
```

B 机 `pull` 后 `docker_* .sh start`（release 仍须挂载二进制）。

---

## 方式三：打成一个压缩包

```bash
cd server-go
bash tools/docker/docker_mirror_save.sh
tar -czf starcrystal-docker-mirror.tar.gz tools/docker/mirror_save tools/docker/docker_*.sh tools/docker/lib tools/docker/release tools/scripts/starcrystal-config.sh
```

B 机解压后 `load` + `docker_startdb.sh` + `docker_release.sh start`（挂载 `starcrystalsvr`）。

---

## 一键安装（开发机）

```bash
cd server-go
bash tools/docker/install-docker.sh
bash tools/docker/docker_svrdev.sh
```

---

## 常见问题

**Q：镜像 tar 在哪？**  
A：仅在 `tools/docker/mirror_save/`（不再写入 `.docker-images/` 或 `offlinesofts/`）。

**Q：release 为什么没有 exe？**  
A：便于跨平台分发静态资源；目标机编译或拷贝对应平台的 `starcrystalsvr` 后 `-v` 挂载。

**Q：容器内 API 如何连 MySQL/Redis？**  
A：`docker_release.sh start` 默认 `host.docker.internal` 指向宿主机上由 `docker_startdb` 拉起的端口。

**Q：防火墙 / 端口？**  
A：默认 `127.0.0.1:3306` / `6379` / `8080`，仅本机访问。
