# network-doctor

Network Doctor is a local network reachability diagnostic CLI. It checks the path from the current machine to a target and explains where a failure is most likely happening.

Network Doctor 是一个本机网络可达性诊断 CLI。它检测当前电脑到目标地址的链路，并尽量定位失败发生在哪一层。

## Highlights / 特性

- Local-machine perspective: DNS, TCP, TLS, HTTP, and selected application protocols are tested from the user's own machine.
- Multi-protocol probes: HTTP/HTTPS, MySQL, Redis, PostgreSQL, SSH, and generic TCP.
- TUN/proxy awareness: detects TUN interfaces, Clash fake-ip DNS, Clash API, and proxy relay failures.
- Layered diagnosis: system, DNS, connection, TLS, protocol, and final suggestions.
- Script friendly: colored text output, JSON output, batch mode, and meaningful exit codes.
- Zero runtime dependency: distributed as a single Go binary.

- **本机视角**：DNS、TCP、TLS、HTTP 和部分应用协议都从用户自己的电脑发起检测。
- **多协议支持**：HTTP/HTTPS、MySQL、Redis、PostgreSQL、SSH、通用 TCP。
- **TUN/代理感知**：识别 TUN 接口、Clash fake-ip DNS、Clash API、代理转发失败。
- **分层诊断**：系统、DNS、连通性、TLS、协议层和最终建议。
- **适合脚本集成**：彩色文本、JSON、批量检测和明确退出码。
- **零运行时依赖**：Go 单二进制分发。

## Install / 安装

### Build from source / 从源码构建

Requires Go 1.19+.

需要 Go 1.19+。

```bash
git clone https://github.com/network-doctor/network-doctor.git
cd network-doctor
go build -o network-doctor .
```

### Cross-platform builds / 跨平台构建

```bash
# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/network-doctor-darwin-arm64 .

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/network-doctor-darwin-amd64 .

# Linux x86_64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/network-doctor-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/network-doctor-linux-arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/network-doctor-windows-amd64.exe .
```

`-ldflags="-s -w"` strips debug symbols and usually reduces binary size.

`-ldflags="-s -w"` 会去除调试符号，通常能减小二进制体积。

## Usage / 使用

```bash
network-doctor <target> [flags]
network-doctor -f <file> [flags]
```

### Examples / 示例

```bash
# HTTPS site / HTTPS 网站
network-doctor https://api.example.com

# Bare domain defaults to HTTPS:443 / 裸域名默认按 HTTPS:443 检测
network-doctor example.com

# Port-based protocol inference / 根据端口自动推断协议
network-doctor db.internal:3306      # MySQL
network-doctor cache.host:6379       # Redis
network-doctor pg.host:5432          # PostgreSQL
network-doctor server:22             # SSH
network-doctor example.com:80        # HTTP
network-doctor example.com:8080      # Generic TCP / 通用 TCP

# Explicit scheme / 显式协议
network-doctor mysql://db.host:3306
network-doctor redis://cache:6379

# IP target skips DNS / IP 目标会跳过 DNS
network-doctor 1.2.3.4:443
```

### Batch mode / 批量检测

```text
# targets.txt
https://api.example.com
mysql://db.internal:3306
redis://cache:6379
10.0.1.5:8080
```

```bash
network-doctor -f targets.txt --concurrency 5
```

Blank lines and lines starting with `#` are ignored.

空行和 `#` 开头的注释行会被忽略。

## Flags / 参数

| Flag | English | 中文 |
|---|---|---|
| `-f, --file <file>` | Read targets from a file | 从文件读取目标列表 |
| `--json` | Output JSON | JSON 格式输出 |
| `--verbose` | Show route, DNS, TLS, and protocol details | 显示路由、DNS、TLS 和协议细节 |
| `--no-color` | Disable colored output | 禁用彩色输出 |
| `--timeout <duration>` | Timeout for each probe, default `10s` | 每个探针的超时时间，默认 `10s` |
| `-c, --concurrency <n>` | Batch concurrency, must be greater than 0 | 批量并发数，必须大于 0 |
| `--clash-api <addr>` | Clash External Controller address | Clash External Controller 地址 |
| `--clash-secret <secret>` | Clash API bearer secret | Clash API 认证密钥 |

## Config / 配置

Network Doctor reads optional defaults from `~/.network_doctor/config`.

Network Doctor 可以从 `~/.network_doctor/config` 读取默认配置。

```text
clash-api: 127.0.0.1:9090
clash-secret: your-secret
```

Priority: command-line flags > config file > automatic discovery.

优先级：命令行参数 > 配置文件 > 自动探测。

## Output Semantics / 输出语义

Probe colors and statuses are intentionally layer-specific.

探针颜色和状态按层级语义区分。

| Status | Meaning | 中文 |
|---|---|---|
| Green / `ok` | The probe succeeded | 当前探针成功 |
| Yellow / `warning` | The layer is reachable, but the result needs attention | 当前层可达，但结果需要关注 |
| Red / `error` | The probe failed and may stop dependent probes | 当前探针失败，后续依赖探针可能跳过 |
| Yellow / `skipped` | The probe was intentionally skipped | 当前探针被有意跳过 |

For HTTP/HTTPS, receiving a non-2xx status still proves that DNS, TCP, TLS, and HTTP reached the server. Therefore `301`, `302`, `401`, `403`, `405`, `500`, and `502` are reported as warnings, not network failures. The final diagnosis remains reachable, with a warning such as `HTTP 403 非 2xx 响应，网络可达但请求未成功`.

对于 HTTP/HTTPS，收到非 2xx 状态码仍然说明 DNS、TCP、TLS 和 HTTP 已经到达目标服务。因此 `301`、`302`、`401`、`403`、`405`、`500`、`502` 会显示为黄色 warning，而不是网络不可达。最终诊断仍然是可达，并附带类似 `HTTP 403 非 2xx 响应，网络可达但请求未成功` 的提示。

## Sample Output / 输出示例

### Reachable target / 目标可达

```text
$ network-doctor https://api.example.com

[系统] 代理: 无 | TUN: 无 | 出口: en0
[DNS]  203.0.113.10 (12ms) | AAAA: 无 | 一致性: ✅
[连通] TCP:443: ✅ 42ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: Let's Encrypt | 证书链: ✅ | 中间人: ✅
[HTTP] 200 OK (85ms) | Server: nginx/1.24

✅ 目标可达
```

### HTTP warning / HTTP 非 2xx

```text
$ network-doctor https://chatgpt.com/

[系统] 代理: 无 | TUN: utun0 (TUN) | 出口: utun4
[代理] Clash (v1.19.21) 代理侧 DNS 解析成功: 103.97.176.73
[DNS]  198.18.0.36 (0ms) | AAAA: 无 | Fake IP ⚠️
[连通] TCP:443: ✅ 0ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: Google Trust Services | 证书链: ✅ | 中间人: ✅
[HTTP] 403 Forbidden (301ms) | Server: cloudflare

✅ 目标可达 (请求经过 TUN 设备 (utun0 (TUN)))
⚠️  代理侧 DNS 解析成功，真实 IP: 103.97.176.73
⚠️  DNS 返回 Fake IP (198.18.x.x)，DNS 被代理接管
⚠️  HTTP 403 非 2xx 响应，网络可达但请求未成功
```

### Clash TUN / Clash TUN 代理环境

When a TUN interface is active, a local TCP handshake may complete against the local proxy even if the proxy cannot relay to the real target. Network Doctor keeps the existing TUN-aware checks:

当 TUN 接口活跃时，本地 TCP 握手可能只是和本地代理完成，并不代表代理能转发到真实目标。Network Doctor 会保留 TUN 感知诊断：

- Detects fake IPs in `198.18.0.0/15`.
- Auto-discovers Clash API at `127.0.0.1:9090` and `127.0.0.1:9097`.
- Queries Clash DNS for real upstream IPs.
- Queries Clash `/connections` to find proxy chains when relay failure is suspected.
- Falls back to protocol behavior, such as EOF or connection reset after sending data.

- 识别 `198.18.0.0/15` fake-ip。
- 自动发现 `127.0.0.1:9090` 和 `127.0.0.1:9097` 上的 Clash API。
- 查询 Clash DNS 获取代理侧真实 IP。
- 在怀疑代理转发失败时查询 Clash `/connections` 获取代理链路。
- 没有 Clash API 时，通过 EOF、connection reset 等协议行为兜底推断。

```text
$ network-doctor 172.36.8.81:8888

[系统] 代理: 无 | TUN: utun0 (TUN) | 出口: utun5
[代理] Clash API 可用 (127.0.0.1:9090)
[DNS]  目标为 IP 地址，跳过 DNS 解析
[连通] TCP:8888: ✅ 0ms
[TLS]  非 TLS 协议，跳过
[TCP] TCP 发送数据后连接断开: EOF

❌ 不可达: TCP:8888 通过代理连接成功，但代理转发到目标失败（目标不可达）
   建议: 1. 检查代理节点是否能访问该目标  2. 尝试切换代理节点或使用直连规则  3. 确认目标地址和端口是否正确
```

### Verbose mode / 详细模式

```text
$ network-doctor --verbose https://api.example.com

[系统] 代理: 无 | TUN: utun3 (TUN) | 出口: en0
       路由: en0 → 192.168.1.1 → 目标
[DNS]  203.0.113.10 (12ms) | AAAA: 无 | 一致性: ✅
       服务器: 114.114.114.114 | 公共DNS: 203.0.113.10
[连通] TCP:443: ✅ 42ms
[TLS]  v1.3 | SNI: ✅ | 颁发者: Let's Encrypt | 证书链: ✅ | 中间人: ✅
       有效期: 2024-01-15 ~ 2025-01-15 (剩余 306 天)
       证书链: Let's Encrypt Authority X3 → ISRG Root X1
       指纹: SHA256:abcdef1234567890...
[HTTP] 200 OK (85ms) | Server: nginx/1.24
       方法: HEAD

✅ 目标可达 (请求经过 TUN 设备 (utun3 (TUN)))
```

## JSON Output / JSON 输出

```bash
network-doctor https://api.example.com --json
```

```json
{
  "target": "https://api.example.com",
  "reachable": true,
  "probes": {
    "system": {
      "name": "system",
      "status": "ok",
      "message": "代理: 无 | TUN: 无 | 出口: en0"
    },
    "dns": {
      "name": "dns",
      "status": "ok",
      "dns": {
        "ipv4": ["203.0.113.10"],
        "consistent": true,
        "public_dns_result": "203.0.113.10"
      }
    },
    "tls": {
      "name": "tls",
      "status": "ok",
      "tls": {
        "version": "v1.3",
        "sni_match": true,
        "valid_chain": true,
        "mitm": false
      }
    },
    "protocol": {
      "name": "protocol",
      "status": "ok",
      "protocol": {
        "type": "http",
        "method": "HEAD",
        "status_code": 200
      }
    }
  },
  "diagnosis": "目标可达"
}
```

Batch mode with `--json` outputs a JSON array.

批量模式下 `--json` 输出 JSON 数组。

## Probe Pipeline / 检测流水线

| Step | Probe | English | 中文 |
|---|---|---|---|
| 1 | `SystemProbe` | System proxy, TUN interface, outbound interface, route | 系统代理、TUN 设备、出口接口、路由 |
| 2 | `ClashProbe` | Clash API discovery and proxy-side DNS | Clash API 探测、代理侧 DNS |
| 3 | `DNSProbe` | A/AAAA records, fake IP, public DNS consistency | A/AAAA、Fake IP、公共 DNS 一致性 |
| 4 | `ConnProbe` | TCP connect, latency, refused/timeout/unreachable classification | TCP 连接、耗时、拒绝/超时/不可达分类 |
| 5 | `TLSProbe` | TLS handshake, SNI, certificate chain, expiry, MITM hints | TLS 握手、SNI、证书链、有效期、中间人提示 |
| 6 | `ProtocolProbe` | HTTP or application protocol handshake, TUN relay failure detection | HTTP 或应用协议握手、TUN 代理转发失败检测 |
| 7 | `Diagnosis` | Summary, warnings, and suggestions | 汇总结论、警告和建议 |

Dependent probes are skipped when an earlier required layer fails.

当前置依赖层失败时，后续探针会自动跳过。

## Protocol Support / 协议支持

| Protocol | Probe behavior | Success / warning semantics |
|---|---|---|
| HTTP/HTTPS | Sends `HEAD`; falls back to `GET` with `Range: bytes=0-0` for `405/501` | `2xx` is OK; non-2xx is warning because HTTP is reachable |
| MySQL | Reads server greeting packet | Version or auth/error response means the service is reachable |
| Redis | Sends `PING` | `PONG` or `NOAUTH` means the service is reachable |
| PostgreSQL | Sends startup message | Any protocol response, including auth rejection, means reachable |
| SSH | Reads server banner | `SSH-...` banner means reachable |
| Generic TCP | Connects and reads optional banner | TCP connect succeeds; TUN mode may send data to detect relay failure |

| 协议 | 探测方式 | 成功 / 警告语义 |
|---|---|---|
| HTTP/HTTPS | 发送 `HEAD`；遇到 `405/501` 时用 `GET` + `Range: bytes=0-0` 兜底 | `2xx` 为 OK；非 2xx 为 warning，因为 HTTP 已到达 |
| MySQL | 读取 server greeting packet | 获取版本或认证/错误响应表示服务可达 |
| Redis | 发送 `PING` | `PONG` 或 `NOAUTH` 表示服务可达 |
| PostgreSQL | 发送 startup message | 收到任意协议响应，包括认证拒绝，表示服务可达 |
| SSH | 读取 server banner | 收到 `SSH-...` banner 表示服务可达 |
| 通用 TCP | 建连并读取可选 banner | TCP 建连成功；TUN 模式下可能主动发数据检测代理转发失败 |

Network Doctor does not perform authentication. Username and password fields in URIs are ignored.

Network Doctor 不执行认证登录。URI 中的用户名和密码字段会被忽略。

## Port Inference / 端口推断

| Port | Scheme |
|---|---|
| 80 | HTTP |
| 443 | HTTPS |
| 3306 | MySQL |
| 5432 | PostgreSQL |
| 6379 | Redis |
| 22 | SSH |
| Other | Generic TCP / 通用 TCP |

Ports must be in the range `1..65535`.

端口必须在 `1..65535` 范围内。

## Exit Codes / 退出码

| Code | English | 中文 |
|---|---|---|
| 0 | Target is network-reachable. Warnings may still exist. | 目标网络可达，可能仍有 warning |
| 1 | Target is unreachable, or any target is unreachable in batch mode. | 目标不可达；批量模式中任一目标不可达 |
| 2 | Invalid arguments, output error, or internal error. | 参数错误、输出错误或工具内部错误 |

```bash
if network-doctor https://api.example.com --no-color > /dev/null 2>&1; then
  echo "network reachable"
else
  echo "network unreachable"
fi
```

## Cross-platform Notes / 跨平台说明

| Feature | macOS | Linux | Windows |
|---|---|---|---|
| System proxy | `scutil --proxy` + environment variables | Environment variables | Registry + environment variables |
| TUN detection | `utun`, `tun`, `tap` | `tun`, `tap`, `wg`, `cali`, `flannel` | `wintun`, `tap` |
| Route detection | `route -n get` | `ip route get` | `route print` |
| DNS server display | `/etc/resolv.conf` | `/etc/resolv.conf` | Not currently shown |
| Colored output | Supported | Supported | Supported in modern terminals |

## Limitations / 限制

- Public DNS consistency uses an external resolver. If that resolver is blocked or times out, the tool reports that consistency is unknown instead of claiming the domain is internal.
- HTTP non-2xx means the HTTP server replied. It is not treated as a TCP/TLS/network failure.
- TUN proxy relay failure detection is best-effort. Clash API improves accuracy, and protocol behavior is used as a fallback.
- Generic TCP cannot know application health without a protocol-specific handshake.

- 公共 DNS 一致性依赖外部解析器。如果外部解析器被阻断或超时，工具会报告一致性未知，而不是直接认定为内部域名。
- HTTP 非 2xx 表示 HTTP 服务已经响应，不会被当成 TCP/TLS/网络失败。
- TUN 代理转发失败检测是尽力判断。Clash API 能提升准确性，协议行为用于兜底。
- 通用 TCP 不做协议握手，无法完整判断应用健康状态。

## License / 许可证

MIT
