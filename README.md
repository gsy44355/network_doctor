# network-doctor

网络可达性诊断 CLI 工具。检测本机到目标地址的连通性，定位不可达的具体原因。

支持多协议（HTTP/HTTPS、MySQL、Redis、PostgreSQL、SSH），输出中文诊断结果，编译为单二进制零依赖分发。

## 安装

### 从源码构建

需要 Go 1.19+：

```bash
git clone https://github.com/network-doctor/network-doctor.git
cd network-doctor
go build -o network-doctor .
```

### 跨平台打包

构建当前平台：

```bash
go build -o network-doctor .
```

交叉编译全平台：

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/network-doctor-darwin-arm64 .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/network-doctor-darwin-amd64 .

# Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o dist/network-doctor-linux-amd64 .

# Linux (ARM64，适用于 ARM 服务器/树莓派)
GOOS=linux GOARCH=arm64 go build -o dist/network-doctor-linux-arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/network-doctor-windows-amd64.exe .
```

一键打包所有平台（可保存为 `build.sh`）：

```bash
#!/bin/bash
VERSION=${1:-"dev"}
DIST="dist"
rm -rf $DIST && mkdir -p $DIST

platforms=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

for platform in "${platforms[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  output="$DIST/network-doctor-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && output+=".exe"

  echo "Building $GOOS/$GOARCH..."
  GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$output" .
done

echo "Done. Binaries in $DIST/"
ls -lh $DIST/
```

> `-ldflags="-s -w"` 会去除调试符号，减小约 30% 的二进制体积。

## 使用方法

```
network-doctor <target> [flags]
network-doctor -f <file>  [flags]
```

### 基本用法

```bash
# HTTPS 站点
network-doctor https://api.example.com

# 指定端口（自动推断协议）
network-doctor db.internal:3306      # → MySQL
network-doctor cache.host:6379       # → Redis
network-doctor pg.host:5432          # → PostgreSQL
network-doctor server:22             # → SSH
network-doctor example.com:80        # → HTTP
network-doctor example.com:8080      # → 通用 TCP

# IP 直连（跳过 DNS 探测）
network-doctor 1.2.3.4:3306

# 裸域名（默认 HTTPS:443）
network-doctor example.com

# 带协议前缀
network-doctor mysql://db.host:3306
network-doctor redis://cache:6379
```

### 批量检测

创建目标文件（`#` 开头为注释，空行忽略）：

```
# targets.txt
https://api.example.com
mysql://db.internal:3306
redis://cache:6379
10.0.1.5:8080
```

```bash
network-doctor -f targets.txt
```

### 命令行选项

| 选项 | 说明 |
|---|---|
| `-f <file>` | 从文件读取目标列表 |
| `--json` | JSON 格式输出 |
| `--verbose` | 显示详细信息（证书链、有效期、路由等） |
| `--no-color` | 禁用彩色输出（也可设置 `NO_COLOR` 环境变量） |
| `--timeout <duration>` | 每个探针的超时时间，默认 `10s` |
| `--clash-api <addr>` | Clash External Controller 地址（如 `127.0.0.1:9090`） |
| `--clash-secret <secret>` | Clash API 认证密钥 |

### 配置文件

支持从 `~/.network_doctor/config` 读取默认配置（文件不存在则忽略，不会自动创建）：

```
# ~/.network_doctor/config
clash-api: 127.0.0.1:9090
clash-secret: your-secret
```

优先级：命令行参数 > 配置文件 > 自动探测。

## 输出示例

### 目标可达

```
$ network-doctor https://api.example.com

[系统] 代理: 无 | TUN: utun3 (TUN) | 出口: en0
[DNS]  203.0.113.10 (12ms) | AAAA: 无 | 一致性: ✅
[连通] TCP:443: ✅ 42ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: Let's Encrypt | 中间人: ✅
[HTTP] 200 OK (85ms) | Server: nginx/1.24

✅ 目标可达 (请求经过 TUN 设备 (utun3 (TUN)))
```

### 代理环境检测（Clash TUN）

当检测到 TUN 设备时，工具会自动识别 Fake IP 并尝试通过 Clash API 查询代理侧的真实 DNS 解析结果：

```
$ network-doctor https://api.example.com

[系统] 代理: 无 | TUN: utun0 (TUN) | 出口: utun6
[代理] Clash (v1.18.0) 代理侧 DNS 解析成功: 93.184.216.34
[DNS]  198.18.2.225 (0ms) | AAAA: 无 | Fake IP ⚠️
[连通] TCP:443: ✅ 0ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: SSL Corporation | 中间人: ✅
[HTTP] 200 OK (156ms) | Server: cloudflare

✅ 目标可达 (请求经过 TUN 设备 (utun0 (TUN)))
⚠️  代理侧 DNS 解析成功，真实 IP: 93.184.216.34
⚠️  DNS 返回 Fake IP (198.18.x.x)，DNS 被代理接管
```

检测能力：

- **Fake IP 识别**：自动检测 DNS 返回的 `198.18.0.0/15` Fake IP 段，提示 DNS 已被代理接管
- **Clash API 查询**：通过 Clash External Controller API 获取代理侧的真实 DNS 解析结果
- **自动发现**：检测到 TUN 设备时自动尝试连接 `127.0.0.1:9090`、`127.0.0.1:9097`
- **手动配置**：可通过 `--clash-api`/`--clash-secret` 参数或配置文件指定

### 目标不可达

```
$ network-doctor mysql://db.internal:3306

[系统] 代理: 无 | TUN: 无 | 出口: en0
[DNS]  10.0.1.50 (5ms) | AAAA: 无 | 内部域名
[连通] TCP:3306: ❌ 连接超时
[MySQL] ⏭️ 跳过 (TCP 不通)

❌ 不可达: TCP:3306 连接超时，可能被防火墙拦截
   建议: 检查安全组/防火墙是否开放 3306 端口
```

### --verbose 模式

```
$ network-doctor --verbose https://api.example.com

[系统] 代理: 无 | TUN: utun3 (TUN) | 出口: en0
       路由: en0 → 192.168.1.1 → 目标
[DNS]  203.0.113.10 (12ms) | AAAA: 无 | 一致性: ✅
       服务器: 114.114.114.114 | 公共DNS: 203.0.113.10
[连通] TCP:443: ✅ 42ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: Let's Encrypt | 中间人: ✅
       有效期: 2024-01-15 ~ 2025-01-15 (剩余 306 天)
       证书链: Let's Encrypt Authority X3 → ISRG Root X1
       指纹: SHA256:ab:cd:ef:12:34:56...
[HTTP] 200 OK (85ms) | Server: nginx/1.24

✅ 目标可达 (请求经过 TUN 设备 (utun3 (TUN)))
```

### --json 模式

```bash
network-doctor https://api.example.com --json
```

```json
{
  "target": "https://api.example.com",
  "reachable": true,
  "probes": {
    "system": { "status": "ok", ... },
    "dns": { "status": "ok", "ipv4": ["203.0.113.10"], ... },
    "conn": { "status": "ok", "port": 443, ... },
    "tls": { "status": "ok", "version": "1.3", "sni_match": true, ... },
    "protocol": { "status": "ok", "type": "http", "status_code": 200, ... }
  },
  "diagnosis": "目标可达",
  "warnings": ["请求经过 TUN 设备 (utun3 (TUN))"]
}
```

批量模式下 `--json` 输出 JSON 数组。

## 检测流水线

按顺序执行，某步骤失败时后续依赖探测自动跳过：

| 步骤 | 探针 | 检测内容 |
|---|---|---|
| 1 | SystemProbe | 系统代理、TUN 设备、出口接口、路由 |
| 2 | ClashProbe | 代理 API 探测、代理侧 DNS 解析（检测到 TUN 时自动启用） |
| 3 | DNSProbe | A/AAAA 记录、Fake IP 检测、DNS 一致性检查（对比公共 DNS） |
| 4 | ConnProbe | TCP 连接、连接耗时、失败分类（拒绝/超时/不可达） |
| 5 | TLSProbe | TLS 握手、SNI 验证、中间人检测（仅 HTTPS） |
| 6 | ProtocolProbe | 协议层握手（HTTP/MySQL/Redis/PostgreSQL/SSH） |
| 7 | Diagnosis | 汇总诊断结论 + 建议 |

## 退出码

| 退出码 | 含义 |
|---|---|
| 0 | 目标可达 |
| 1 | 目标不可达（或批量模式中任一目标不可达） |
| 2 | 参数错误或工具内部错误 |

可在脚本中使用退出码进行自动化判断：

```bash
if network-doctor https://api.example.com --no-color > /dev/null 2>&1; then
  echo "服务可达"
else
  echo "服务不可达"
fi
```

## 协议支持

| 协议 | 握手方式 | 成功判定 |
|---|---|---|
| HTTP/HTTPS | HEAD 请求 | 收到任何 HTTP 响应（含 4xx/5xx） |
| MySQL | 读取 server greeting packet | 获取到版本号 |
| Redis | 发送 PING | 收到 PONG 或 -NOAUTH |
| PostgreSQL | 发送 startup message | 收到任何响应（含认证拒绝） |
| SSH | 读取 server banner | 获取到版本字符串 |
| 通用 TCP | 连接后读 banner（超时内） | 连接成功即通过 |

> 工具仅做协议层可达性检测，不执行认证登录。URI 中的用户名/密码字段会被忽略。

## 端口推断规则

未指定协议前缀时，按端口号自动推断：

| 端口 | 推断协议 |
|---|---|
| 80 | HTTP |
| 443 | HTTPS |
| 3306 | MySQL |
| 5432 | PostgreSQL |
| 6379 | Redis |
| 22 | SSH |
| 其他 | 通用 TCP |

## 跨平台支持

| 功能 | macOS | Linux | Windows |
|---|---|---|---|
| 系统代理检测 | `scutil --proxy` + 环境变量 | 环境变量 | 注册表 + 环境变量 |
| TUN 设备检测 | utun/tun 接口 | tun 接口 | wintun/tap 接口 |
| 路由检测 | `route -n get` | `ip route get` | `route print` |
| DNS 服务器 | `/etc/resolv.conf` | `/etc/resolv.conf` | — |
| 彩色输出 | 原生支持 | 原生支持 | Windows Terminal 支持 |

## License

MIT
