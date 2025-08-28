# API Mode Example

这个示例展示了如何使用 Glider 的新 API 控制模式。

## 功能特性

### API 负载均衡策略
- **策略名称**: `api`
- **工作机制**: 
  - 使用全局变量 `current_proxy` 来控制当前使用的代理
  - 如果 `current_proxy` 为 `nil`，则随机选择一个代理
  - 如果 `current_proxy` 不为 `nil`，则直接使用该代理

### HTTP API 接口
当启用 API 服务器时（设置 `serverPort` 参数），Glider 会提供以下 HTTP 接口：

#### 1. 切换代理 - POST /api/proxy/change
随机切换到不同的代理服务器。

**请求方法**: `POST`
**URL**: `http://localhost:9000/api/proxy/change`

**响应示例**:
```json
{
  "success": true,
  "message": "Proxy changed successfully",
  "current_proxy": {
    "address": "proxy2.example.com:1080",
    "priority": 0,
    "enabled": true,
    "latency": 120
  }
}
```

#### 2. 获取当前代理 - GET /api/proxy/current
获取当前正在使用的代理信息。

**请求方法**: `GET`
**URL**: `http://localhost:9000/api/proxy/current`

**响应示例**:
```json
{
  "success": true,
  "message": "Current proxy retrieved successfully",
  "current_proxy": {
    "address": "proxy1.example.com:1080",
    "priority": 0,
    "enabled": true,
    "latency": 95
  }
}
```

#### 3. 获取代理列表 - GET /api/proxy/list
获取所有可用的代理服务器列表。

**请求方法**: `GET`
**URL**: `http://localhost:9000/api/proxy/list`

**响应示例**:
```json
{
  "success": true,
  "message": "Proxy list retrieved successfully",
  "proxy_list": [
    {
      "address": "proxy1.example.com:1080",
      "priority": 0,
      "enabled": true,
      "latency": 95
    },
    {
      "address": "proxy2.example.com:1080",
      "priority": 0,
      "enabled": true,
      "latency": 120
    },
    {
      "address": "proxy3.example.com:1080",
      "priority": 0,
      "enabled": false,
      "latency": 0
    }
  ]
}
```

## 使用方法

### 1. 启动 Glider
```bash
glider -config glider.conf
```

### 2. 使用 curl 测试 API

**切换代理**:
```bash
curl -X POST http://localhost:9000/api/proxy/change
```

**查看当前代理**:
```bash
curl http://localhost:9000/api/proxy/current
```

**查看所有代理**:
```bash
curl http://localhost:9000/api/proxy/list
```

### 3. 配置要点

在配置文件中需要设置：

```conf
# 使用 API 策略
strategy=api

# 启用 API 服务器
serverPort=9000

# 配置多个代理
forward=socks5://proxy1.example.com:1080
forward=socks5://proxy2.example.com:1080
forward=socks5://proxy3.example.com:1080
```

## 切换逻辑

API 接口 `/api/proxy/change` 的切换逻辑：

1. 从可用代理列表中随机选择一个代理
2. 如果选中的代理与当前代理不同，立即使用
3. 如果选中的代理与当前代理相同，最多重试 3 次
4. 3 次重试后仍然相同，则使用最后选中的代理

这确保了在大多数情况下能够切换到不同的代理，同时避免无限循环。

## 应用场景

- **动态代理切换**: 根据网络状况或需求动态切换代理
- **外部控制**: 通过外部脚本或程序控制代理选择
- **负载测试**: 快速切换不同代理进行测试
- **故障恢复**: 当前代理出现问题时快速切换
