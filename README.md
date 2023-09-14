# wxpush

## 用法

### Docker

本地构建镜像：
`docker build -t local/wxpush:latest .`

运行:
`docker run -d --name wxpush -p 10000:10000 -v /local/path/to/config.yaml:/root/config.yaml local/wxpush`

### 二进制

本地编译：
`go build -o wxpush`

运行:
`./wxpush`

### 其他

访问 `http://ip:port/ip` 可获取外网ip列表