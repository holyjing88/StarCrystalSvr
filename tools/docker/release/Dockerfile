# StarCrystal release 运行包（configs + assets + 启停脚本；不含 starcrystalsvr 二进制）
FROM alpine:3.20
RUN apk add --no-cache bash python3
WORKDIR /app
COPY startsvr.sh stopsvr.sh ./
COPY configs ./configs
COPY assets ./assets
RUN mkdir -p log && chmod 777 log
# starcrystalsvr / starcrystalsvr.exe 由 docker run -v 挂载到 /app/starcrystalsvr
ENV AUTH_SMS_MOCK=1
EXPOSE 8080
CMD ["./startsvr.sh"]
