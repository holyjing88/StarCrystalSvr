StarCrystal 发布打包

====================================================



**权威位置（唯一发布入口）**



    Y:\holyjing\starcrystalsvr\tools\0publish\scripts\



勿在 `d:\0_games\000StarCrystal\server` 下发布。开发机改脚本后执行:



    .\tools\scripts\sync-publish-scripts-to-y.ps1



Windows 发布

------------

    cd Y:\holyjing\starcrystalsvr

    .\tools\0publish\scripts\pack-publish.ps1



输出

----

  子目录 `tools/0publish/yyyyMMdd-HHmmss/`（与 scripts 同级）:

    release.zip / release.tar.gz

    dbscripts.zip / dbscripts.tar.gz

    idip-webclient.zip / idip-webclient.tar.gz

    release_h5.zip / release_h5.tar.gz   （H5 小游戏静态与整包，源：仓库根 release_h5/）

    pack-manifest.txt

    unpack.sh                   （外网在子目录内执行，部署到 /app/publish + CDN /h5）

  设计说明（release_h5 入包与 CDN 解包）:

    tools/0publish/doc/发布包release_h5与CDN解包设计.md



  子目录打总包（放在 `tools/0publish/` 当前目录下）:

    yyyyMMdd-HHmmss.zip    （内含上述子目录全部文件）



Linux

-----

    bash tools/0publish/scripts/pack-publish.sh

    → 子目录内四个 .tar.gz + 根目录 `yyyyMMdd-HHmmss.tar.gz`



    bash tools/0publish/scripts/pack-publish-verify.sh --latest



外网部署（/app/publish）:

  将总包解压到 /app/publish/yyyyMMdd-HHmmss/ 后:

    cd /app/publish/yyyyMMdd-HHmmss

    bash unpack.sh

  备份并删除 /app/publish/{release,dbscripts,idip-webclient}，
  再用当前子目录内压缩包解压并移到 /app/publish/。

  随后：备份 /wwwroot/minigame.starlaneinfinite.com/h5（最多保留 10 份），
    清空 h5/ 下全部内容，
    将 release_h5 压缩包解压到 h5/ 下（与 IDIP/rsync 目标一致）。

  可选: UNPACK_DEPLOY_ROOT=/app/publish bash unpack.sh --dry-run
  可选: UNPACK_SKIP_CDN_H5=1 bash unpack.sh   （仅部署 API，不动 CDN）



远程 Linux: bash tools/0publish/scripts/pack-publish-linux-remote.sh



可选: -SkipBuild, -BuildIdip, -PublishDir yyyyMMdd-HHmmss


