# Glider 代理池模式技术方案

## 一、背景

### 现有架构

当前系统通过 **sx.org** 代理服务实现代理功能，整体调用链路如下：

```
前端 (toolbox-react) → Toolbox 后端 (Java) → Glider (Go) → sx.org API
```

Glider 部署在 AWS EC2 上，通过 sx.org 的 API Key 调用其服务。

### 现有代理模式（模式1：固定代理刷新 IP）

- 通过 sx.org API 创建固定代理
- 配置到 Glider 的 `glider.conf` 中
- 长期使用同一个代理链接
- IP 用完后调用 refresh 接口刷新

```
本地 → AWS Glider (:10801) → 固定代理 (socks5://固定IP) → 目标网站
                                    │
                                    ▼
                              需要时调用 refresh API 刷新 IP
```

---

## 二、新需求

### 模式2：代理池（用完即弃）

sx.org 还提供另一种使用方式：

1. 通过 URL 批量获取代理列表（如一次获取 100-2000 个）
2. 每个代理对应一个独立的 IP
3. 用完即弃，不需要刷新
4. 代理用完后，再从 URL 获取新的一批

**sx.org 代理列表 URL 示例：**

```
https://sx-list.org/5VTRbxsTzhF38vuh1Uxr6LElPIzZmOhs.txt?limit=100&type=res&use_login_and_password=1&template_id=4&country=US
```

**返回格式（每行一个代理）：**

```
http://u1:p1@89.38.99.101:9999
http://u2:p2@89.38.99.102:9999
http://u3:p3@89.38.99.103:9999
...
```

---

## 三、核心问题

### 风控检测

如果本地直接使用 sx.org 的代理：

```
❌ 错误方式：
本地 IP (123.45.67.89) ──直连──▶ sx 代理 ──▶ 目标网站
                                   │
                                   ▼
                            sx.org 记录：
                            "代理被 IP 123.45.67.89 使用"
```

**问题：**
- sx.org 会检测到本地 IP
- 如果同一个 IP 使用多个不同账号的代理 → 触发风控 → 封禁/吊销 Plan

### 解决思路

所有请求必须经过 Glider 中转：

```
✅ 正确方式：
本地 IP ──▶ AWS Glider ──▶ sx 代理 ──▶ 目标网站
                │
                ▼
         sx.org 只看到 AWS IP
         无法关联到本地 IP
```

---

## 四、需求分析

### 关键需求

1. **批量获取代理**：从 sx-list.org 获取 100 个代理
2. **经过 Glider 转发**：避免暴露本地 IP
3. **软件无需改动**：返回的代理格式与 sx 原始格式兼容
4. **无状态设计**：Glider 重启不受影响

### 挑战

如果使用映射存储方案：
- 需要维护 pool-1 → proxy1 的映射关系
- Glider 重启需要恢复机制
- 需要同步机制保持 Toolbox 和 Glider 数据一致

---

## 五、解决方案：Base64 动态代理

### 核心思路

将代理信息编码在用户名中，Glider 无需存储任何映射：

```
原始代理:     socks5://user:pass@ip:port
              ↓ Base64 编码
用户名:       c29ja3M1Oi8vdXNlcjpwYXNzQGlwOnBvcnQ=
              ↓ 组装
Glider代理:   http://{base64}:fixed_password@glider:10800
```

### 方案优势

| 方面 | 映射存储方案 | Base64 方案 |
|------|------------|-------------|
| 存储 | 需要内存/文件/数据库 | 无需存储 ✓ |
| 状态 | 有状态，需要同步 | 完全无状态 ✓ |
| 重启 | 需要恢复机制 | 无影响 ✓ |
| 扩展 | 需要提前同步映射 | 即时支持任意代理 ✓ |
| 代码改动 | 较多 | 只需加一个解码逻辑 ✓ |

---

## 六、架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           Base64 动态代理架构                                        │
└─────────────────────────────────────────────────────────────────────────────────────┘

1. 获取代理列表（Toolbox 处理）:

   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │  sx-list.org 返回:                    Toolbox 转换后返回:                         │
   │                                                                                 │
   │  socks5://u1:p1@ip1:port1   ──▶   http://c29ja3M1Oi8vdTE...@glider:10800        │
   │  socks5://u2:p2@ip2:port2   ──▶   http://c29ja3M1Oi8vdTI...@glider:10800        │
   │  socks5://u3:p3@ip3:port3   ──▶   http://c29ja3M1Oi8vdTM...@glider:10800        │
   │  ...                                                                            │
   └─────────────────────────────────────────────────────────────────────────────────┘

2. 软件使用代理:

   ┌──────────────┐              ┌───────────────────────────────────────────────────┐
   │   软件        │              │              AWS Glider                           │
   │              │              │                                                   │
   │  使用代理:    │  ──────────▶ │  收到: http://c29ja3M1...:secret@:10800           │
   │  c29ja3M1..  │              │                                                   │
   │  :secret     │              │  1. 检测密码 = 固定密码 ✓                          │
   │  @glider     │              │  2. Base64 解码用户名                             │
   │  :10800      │              │     → socks5://u1:p1@ip1:port1                   │
   │              │              │  3. 通过这个代理转发请求                           │
   └──────────────┘              └───────────────────────────────────────────────────┘
```

### 请求流程

```
用户软件
    │
    │ 使用代理: http://c29ja3M1...==:secret@52.xx.xx.xx:10800
    ▼
AWS Glider (:10800)
    │
    ├─ 1. 解析 HTTP Basic Auth
    ├─ 2. 检测密码是否为固定密码（触发动态模式）
    ├─ 3. Base64 解码用户名 → socks5://u1:p1@ip1:port1
    ├─ 4. 动态创建 Dialer
    ├─ 5. 通过该代理转发请求
    │
    ▼
sx.org 代理 (socks5://u1:p1@ip1:port1)
    │
    │ sx.org 看到的是 AWS IP，不是用户本地 IP ✓
    ▼
目标网站
```

---

## 七、API 设计

### Toolbox 新增 API

#### 1. 获取动态代理列表

```
GET /toolbox/admin/proxy/pool/dynamic/list

Query Parameters:
  - poolUrl: sx-list.org 的代理列表 URL
  - format: 返回格式（plain / json）

Response (plain):
http://c29ja3M1Oi8vdTE6cDE...@52.xx.xx.xx:10800
http://c29ja3M1Oi8vdTI6cDI...@52.xx.xx.xx:10800
http://c29ja3M1Oi8vdTM6cDM...@52.xx.xx.xx:10800
...

Response (json):
{
    "success": true,
    "data": {
        "proxies": [
            "http://c29ja3M1Oi8vdTE6cDE...@52.xx.xx.xx:10800",
            "http://c29ja3M1Oi8vdTI6cDI...@52.xx.xx.xx:10800"
        ],
        "total": 100,
        "gliderHost": "52.xx.xx.xx",
        "gliderPort": 10800
    }
}
```

### Glider 改动

#### 动态代理处理逻辑

Glider 的 HTTP Proxy 处理器中增加：

1. 解析 HTTP Basic Auth 获取 username 和 password
2. 如果 password 等于预设的固定密码 → 进入动态模式
3. Base64 解码 username → 得到真实代理 URL
4. 动态创建 Dialer 并转发请求

---

## 八、配置说明

### Glider 配置

```conf
# glider.conf

# 基础配置
verbose=True
serverPort=6777

# SXX API 配置
sxxhost=https://api.sx.org
sxxkey=your_sxx_auth_key

# ========== 动态代理模式 ==========
# 单端口处理所有动态代理请求
listen1=http://:10800
# 不需要 forward，由动态模式处理

# ========== 固定代理模式（可选，与动态模式共存）==========
listen2=http://user2:pass2@:10801
forward2=socks5://fixed_user:fixed_pass@89.38.99.100:9999
```

### 动态模式配置项

```conf
# 动态代理模式的固定密码
dynamicProxyPassword=sx_dynamic_proxy_secret

# 动态代理监听端口
dynamicProxyPort=10800
```

---

## 九、使用示例

### Python 示例

```python
import base64
import requests

# 1. 原始 sx 代理
original_proxy = "socks5://nnubvxka:z5of2vzxssfg@208.66.74.184:5499"

# 2. Base64 编码
encoded = base64.b64encode(original_proxy.encode()).decode()

# 3. 组装 Glider 代理
glider_host = "52.xx.xx.xx"
glider_port = 10800
fixed_password = "sx_dynamic_proxy_secret"

glider_proxy = f"http://{encoded}:{fixed_password}@{glider_host}:{glider_port}"

# 4. 直接使用（与原始 sx 代理格式兼容）
response = requests.get("https://httpbin.org/ip", proxies={
    "http": glider_proxy,
    "https": glider_proxy
})
```

### 批量使用

```python
# 从 Toolbox 获取转换后的代理列表
response = requests.get(
    "http://toolbox/admin/proxy/pool/dynamic/list",
    params={
        "poolUrl": "https://sx-list.org/xxx.txt?limit=100",
        "format": "plain"
    }
)

proxy_list = response.text.strip().split("\n")
# ['http://c29ja3M1...@glider:10800', 'http://c29ja3M1...@glider:10800', ...]

# 直接配置到软件，格式与 sx 原始代理兼容
for proxy in proxy_list:
    do_something_with_proxy(proxy)
```

---

## 十、性能分析

### 开销分解

```
创建 Dialer 对象:        ~100 纳秒 (内存分配)
Base64 解码:             ~1 微秒
TCP 连接到代理服务器:     ~50-200 毫秒 (网络 IO，主要开销)
代理握手 (SOCKS5/HTTP):   ~10-50 毫秒
TCP 连接到目标网站:       ~50-200 毫秒 (网络 IO，主要开销)
```

### 结论

- 99.9% 的开销在网络 IO
- Dialer 创建/销毁开销可忽略不计（< 1 微秒）
- 对于"用完即弃"的代理，动态创建 Dialer 是合理的

---

## 十一、两种模式对比

| 特性 | 模式1（固定代理刷新 IP）| 模式2（代理池用完即弃）|
|------|----------------------|---------------------|
| 代理获取 | API 创建固定代理 | URL 批量拉取 |
| 使用方式 | 长期复用，刷新 IP | 用完即弃 |
| 代理数量 | 少量（个位数）| 大量（100-2000）|
| 适用场景 | 稳定需求 | 批量一次性任务 |
| Glider 配置 | 需要预配置 forward | 动态解析，无需预配置 |
| 状态 | 有状态（配置文件）| 无状态 |

---

## 十二、实现计划

### Phase 1: Glider 改动

1. 新增动态代理处理逻辑
2. 支持 Base64 解码用户名
3. 动态创建 Dialer 并转发
4. 与现有配置文件模式兼容

### Phase 2: Toolbox 改动

1. 新增 `/pool/dynamic/list` API
2. 从 sx-list.org 拉取代理
3. Base64 编码并组装 Glider 代理格式
4. 返回给客户端

### Phase 3: 前端改动（可选）

1. SxProxy 页面新增"代理池模式"Tab
2. 配置代理池 URL
3. 预览/复制转换后的代理列表

---

## 十三、总结

**一句话总结：**

> 将代理信息 Base64 编码到用户名中，Glider 只需解码转发，完全无状态，软件无需改动。

**核心价值：**

1. ✅ 避免 sx.org 风控检测
2. ✅ Glider 完全无状态，重启无影响
3. ✅ 软件使用方式与 sx 原始代理兼容
4. ✅ 支持大批量"用完即弃"代理

