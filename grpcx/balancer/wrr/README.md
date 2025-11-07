# VIP分组负载均衡器

这是一个支持VIP分组的平滑加权轮询负载均衡器，可以为VIP用户提供优先服务。

## 功能特性

- ✅ 平滑加权轮询算法
- ✅ VIP用户优先路由
- ✅ 自动降级机制
- ✅ 动态权重调整
- ✅ 多种VIP识别方式

## VIP分组策略

### 路由优先级

1. **VIP用户** → **VIP分组节点**（最高优先级）
2. **VIP用户** → **普通分组节点**（VIP分组不可用时降级）
3. **普通用户** → **普通分组节点** (普通用户不进行降级处理)

### VIP识别方式

1. **请求头中的VIP标识**：`vip=true`
2. **请求头中的用户类型**：`user-type=vip`
3. **请求头中的用户ID**：以 `V` 开头的用户ID

## 使用方法

### 1. 服务端注册

#### VIP节点注册
```go
// 注册VIP服务端节点
err = em.AddEndpoint(ctx, key, endpoints.Endpoint{
    Addr: addr,
    Metadata: map[string]any{
        "weight": 20,        // VIP节点权重更高
        "group":  "vip",     // VIP分组
        "region": "us-east", // 其他元数据
    },
}, etcdv3.WithLease(leaseResp.ID))
```

#### 普通节点注册
```go
// 注册普通服务端节点
err = em.AddEndpoint(ctx, key, endpoints.Endpoint{
    Addr: addr,
    Metadata: map[string]any{
        "weight": 10,        // 普通节点权重较低
        "group":  "normal",  // 普通分组
        "region": "us-east", // 其他元数据
    },
}, etcdv3.WithLease(leaseResp.ID))
```

### 2. 客户端配置

```go
import _ "github.com/Kirby980/go-pkg/grpcx/balancer/wrr"

cc, err := grpc.Dial("etcd:///service/user",
    grpc.WithResolvers(bd),
    grpc.WithDefaultServiceConfig(`{
        "loadBalancingConfig": [{"smooth_weight_round_robin": {}}]
    }`),
    grpc.WithTransportCredentials(insecure.NewCredentials()))
```

### 3. VIP用户请求

#### 方法1: 通过VIP标识
```go
ctx := metadata.AppendToOutgoingContext(context.Background(), "vip", "true")
resp, err := client.Select(ctx, &userv1.SelectRequest{Id: 123})
```

#### 方法2: 通过用户类型
```go
ctx := metadata.AppendToOutgoingContext(context.Background(), "user-type", "vip")
resp, err := client.Select(ctx, &userv1.SelectRequest{Id: 456})
```

#### 方法3: 通过用户ID
```go
ctx := metadata.AppendToOutgoingContext(context.Background(), "user-id", "V12345")
resp, err := client.Select(ctx, &userv1.SelectRequest{Id: 789})
```

### 4. 普通用户请求
```go
// 不添加任何VIP标识，自动使用普通分组
resp, err := client.Select(context.Background(), &userv1.SelectRequest{Id: 999})
```

## 应用场景

### 场景1: 电商平台
```go
// VIP用户享受优先服务
// 普通用户使用常规服务
// VIP分组节点配置更高性能的服务器
```

### 场景2: 游戏服务器
```go
// VIP玩家优先匹配到VIP服务器
// 普通玩家使用普通服务器
// 保证VIP玩家的游戏体验
```

### 场景3: 金融系统
```go
// VIP客户优先处理交易
// 普通客户使用常规处理流程
// 确保重要客户的交易优先级
```

## 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| weight | int | 10 | 节点权重 |
| group | string | "normal" | 分组名称（vip/normal） |

## 故障转移机制

- **VIP分组不可用**：自动降级到普通分组
- **普通分组不可用**：从所有可用节点中选择
- **动态权重调整**：根据请求结果调整节点权重
- **健康检查**：自动剔除不健康的节点

## 监控建议

1. **分组分布监控**：监控VIP和普通请求的分布情况
2. **响应时间监控**：分别监控VIP和普通用户的响应时间
3. **降级频率监控**：监控VIP降级到普通分组的频率
4. **节点健康监控**：监控各分组节点的健康状态

## 注意事项

1. VIP分组节点建议配置更高性能的服务器
2. 普通分组节点数量应该足够支撑降级流量
3. 建议设置合理的权重比例（VIP:普通 = 2:1）
4. 监控VIP降级情况，及时扩容VIP分组
5. 定期评估VIP用户的服务质量 