联网机 fetch 后生成：

  gomod-cache.tar.gz

离线 install 解压到 server-go/.gomodcache，编译时：

  export GOMODCACHE=.../server-go/.gomodcache GOPROXY=off
