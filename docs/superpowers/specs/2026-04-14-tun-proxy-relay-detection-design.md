# TUN 透明代理转发失败检测

## 问题

当本机使用 Clash 等透明代理（TUN 模式）时，`ConnProbe` 的 TCP 三次握手实际上与本地 TUN 接口完成，而非真实目标。即使目标不可达，ConnProbe 也报告 `StatusOK`，导致最终误诊为"可达"。

代理在转发失败时的行为是：TCP 握手成功后，代理尝试连接真实目标失败，直接关闭连接返回 EOF（空响应）。所有协议（HTTP、MySQL、Redis 等）表现一致，因为本质都是 TCP 转发失败。

## 设计目标

- TUN 活跃时，不盲信 ConnProbe 的 TCP 成功结果
- 两层验证：Clash API 精确查询优先，行为推断兜底
- 代理转发失败时标记为不可达，并区分"代理转发失败"和"直连不可达"两种原因，给出不同建议
- 没有 TUN 时所有行为完全不变

## 方案

### 整体架构

管线顺序不变。ProtocolProbe 内部逻辑增强（两层检测都在此阶段执行），Diagnosis 引擎增加代理转发失败诊断规则。

```
SystemProbe → ClashProbe(DNS) → DNSProbe → ConnProbe → TLSProbe → ProtocolProbe → Diagnosis
                                                                       |
                                                                [TCP 成功 + TUN 活跃?]
                                                                       | 是
                                                                第一层: 通过 Clash API 查连接状态
                                                                       | API 不可用
                                                                第二层: 协议层 EOF 行为推断
```

**时序说明：** ClashProbe 在管线前段运行，负责 DNS 查询（已有功能）。连接状态检查在 ProtocolProbe 阶段执行——此时 ConnProbe 已完成 TCP 连接，ProtocolProbe 会建立自己的 TCP 连接进行协议握手，可以在此连接存活期间查询 Clash API `/connections`。ProtocolProbe 通过 `prev["clash"]` 获取 Clash API 地址和认证信息。

### 第一层：Clash API 连接状态查询（精确检测）

**触发条件：** ConnProbe 返回 StatusOK + SystemProbe 检测到 TUN 活跃 + ClashProbe 的 API 可用。

**实现：** 在 ProtocolProbe 中，协议握手之前，调用 Clash API 查询连接状态：

1. 从 `prev["clash"]` 获取 API 地址和 secret
2. ProtocolProbe 建立 TCP 连接后（握手之前），调用 `GET /connections` 获取活跃连接列表
3. 在连接列表中查找匹配 `host:port` 的连接（匹配 `metadata.host` 或 `metadata.destinationIP` + `metadata.destinationPort`）
4. 检查该连接的 `chains` 字段获取代理链路
5. 然后执行正常协议握手，根据握手结果判断转发是否成功

**ClashDetails 新增字段：**

```go
type ClashDetails struct {
    // 已有字段
    Available  bool
    APIAddr    string
    Version    string
    RealIPs    []string
    DNSSuccess bool
    DNSError   string

    // 新增
    ProxyRelay  bool     // 流量是否经过代理转发（非 DIRECT）
    ProxyChain  []string // 代理链路，如 ["Proxy", "HK-Node"]
    RelayFailed bool     // Clash API 确认代理转发失败
}
```

**注意：** ProtocolProbe 不直接修改 `ClashDetails`。新增字段通过 `ProtocolDetails` 记录 Clash API 查询结果，Diagnosis 引擎综合判断。具体方式：ProtocolProbe 中新增辅助字段记录从 Clash API 获取的代理链路信息。

**判断逻辑：**

- API 查到匹配连接 + 协议握手收到 EOF → 精确确认代理转发失败
- API 查到匹配连接 + 协议握手成功 → 代理转发成功，记录代理链路
- API 不可用 → 交给第二层行为推断

### 第二层：ProtocolProbe 行为推断（Fallback）

**触发条件：** TUN 活跃 + Clash API 不可用或未找到匹配连接。

**检测逻辑：** 在 ProtocolProbe 中，对协议探测失败模式做特殊分类。与第一层在同一个 ProtocolProbe 执行流程中，第一层查不到时自然进入第二层。

**ProtocolDetails 新增字段：**

```go
type ProtocolDetails struct {
    // 已有字段
    Type         string
    StatusCode   int
    Version      string
    Banner       string
    AuthRequired bool
    Error        string

    // 新增
    ProxyRelayFailed bool // 推断为代理转发失败
}
```

**推断规则（所有协议统一）：**

- 连接成功后立即收到 EOF / 连接关闭 + TUN 活跃 → `ProxyRelayFailed = true`
- 具体表现：
  - HTTP/HTTPS：发送请求后收到 EOF（空响应）
  - MySQL/Redis/PostgreSQL/SSH/GenericTCP：连接后握手阶段收到 EOF

**关键约束：** 只有 TUN 活跃时才做此推断。没有 TUN 时，EOF 仍按原逻辑处理（服务异常）。

### Diagnosis 引擎增强

在 `diagnosis.Diagnose()` 中，TCP 连接评估之后、TLS 评估之前，新增代理转发失败诊断分支。

**判断优先级：**

1. `ClashDetails.ConnectionClosed == true` → 精确确认代理转发失败
2. `ProtocolDetails.ProxyRelayFailed == true` → 行为推断代理转发失败

**诊断输出：**

```
Reachable: false
Summary: "TCP:{port} 通过代理连接成功，但代理转发到目标失败（目标不可达）"
Suggestion: "1. 检查代理节点是否能访问该目标  2. 尝试切换代理节点或使用直连规则  3. 确认目标地址和端口是否正确"
```

如果 ClashDetails 中有 `ProxyChain` 信息，额外输出代理链路：

```
Suggestion: "当前代理链路: Proxy → HK-Node，建议切换其他节点或添加直连规则"
```

### 不变的部分

- 管线顺序不变
- 没有 TUN 时所有逻辑完全不变
- 现有 Fake IP 检测、DNS 一致性检查保持不变
- JSON 输出结构向后兼容（只新增字段）
- CLI 参数不变（复用 `--clash-api` 和 `--clash-secret`）
- 退出码语义不变
