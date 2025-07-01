# Go Pkg Library

这是一个Go语言的通用工具库，包含了微服务开发中常用的各种工具和组件。

## 功能特性

- **grpcx**: gRPC相关工具，包括负载均衡器、熔断机制、日志、prometheus可视化、trace链路追踪等
- **ginx**: Gin框架扩展，包含redis限流中间件、prometheus可视化、自定义日志、服务器包装等
- **gormx**: GORM扩展，包含缓存、连接池、查询构建器、双写机制(可以指定源、目标数据库)、读写分离等
- **logger**: 日志工具，支持结构化日志和全局实例
- **redisx**: Redis扩展，包含OpenTelemetry和Prometheus集成
- **saramax**: Sarama Kafka客户端扩展，支持批量生产者传递、转递结构体到kafka、批量消费者模式等
- **ratelimit**: 限流工具，支持Redis滑动窗口算法
- **migrator**: 数据库迁移工具、支持同源数据库不停机迁移。有增量同步、全量同步；支持切换源表目标表切换、双写和校验、自定义比较方法等
- **cronx**: 定时任务扩展
- **netx**: 网络工具，如IP地址获取等

## 安装

```bash
go get github.com/Kirby980/go-pkg
```

## 使用示例

### gRPC负载均衡

```go
import "github.com/Kirby980/go-pkg/grpcx"

// 使用自定义负载均衡器
cc, err := grpc.Dial("etcd:///service/user",
    grpc.WithDefaultServiceConfig(`{
        "loadBalancingConfig": [
            {
                "smooth_weight_round_robin": {}
            }
        ]
    }`))
```

### Gin中间件

```go
import "github.com/Kirby980/go-pkg/ginx"

server := ginx.NewServer()
server.Use(ginx.Logger())
server.Use(ginx.RateLimit())
```

### 数据库操作

```go
import "github.com/Kirby980/go-pkg/gormx"

// 使用缓存查询
repo := gormx.NewRepository(db)
result, err := repo.QueryWithCache(ctx, "cache_key", func() (interface{}, error) {
    return userService.GetUser(id)
})
```

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

MIT License 