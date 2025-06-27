package wrr

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
)

// smooth_weight_round_robin 平滑加权轮询算法
const Name = "smooth_weight_round_robin"

func init() {
	balancer.Register(newBuilder())
}

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &PickerBuilder{}, base.Config{HealthCheck: true})
}

type PickerBuilder struct {
}

func (w *PickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	grpclog.Infof("smoothWeightRoundRobin: Build called with info: %v", info)

	// 按分组组织连接
	groupConns := make(map[string][]*conn)

	for sc, sci := range info.ReadySCs {
		cc := &conn{
			cc: sc,
		}
		md, ok := sci.Address.Metadata.(map[string]any)
		if ok {
			weightVal := md["weight"]
			weight, _ := weightVal.(float64)
			cc.weight = int(weight)
			if md["group"] != nil {
				cc.group = md["group"].(string)
			}
		}
		if cc.weight == 0 {
			cc.weight = 10
		}
		if cc.group == "" {
			cc.group = "default" // 默认分组
		}
		cc.currentWeight = cc.weight

		// 按分组收集连接
		groupConns[cc.group] = append(groupConns[cc.group], cc)
	}

	return &picker{
		groupConns: groupConns,
	}
}

type picker struct {
	groupConns map[string][]*conn
	mutex      sync.Mutex
}

// Pick 实现VIP优先的负载均衡
func (p *picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.groupConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// 从请求上下文中获取VIP标识
	isVIP := p.isVIPRequest(info)

	// VIP用户优先使用VIP分组
	if isVIP {
		if vipConns, exists := p.groupConns["vip"]; exists && len(vipConns) > 0 {
			best := p.pickFromGroup(vipConns)
			return balancer.PickResult{
				SubConn: best.cc,
				Done: func(di balancer.DoneInfo) {
				},
			}, nil
		}
		// VIP分组没有可用节点，降级到普通分组
	}

	// 普通用户或VIP降级：优先使用普通分组
	if normalConns, exists := p.groupConns["normal"]; exists && len(normalConns) > 0 {
		best := p.pickFromGroup(normalConns)
		return balancer.PickResult{
			SubConn: best.cc,
			Done: func(di balancer.DoneInfo) {
			},
		}, nil
	} else {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
}

// isVIPRequest 判断是否为VIP请求
func (p *picker) isVIPRequest(info balancer.PickInfo) bool {
	// 从请求的元数据中获取VIP标识
	if md, ok := metadata.FromOutgoingContext(info.Ctx); ok {
		// 方法1: 检查vip字段
		if vipValues := md.Get("vip"); len(vipValues) > 0 && vipValues[0] == "true" {
			return true
		}
	}

	// 方法2: 从请求头中获取用户类型
	if userType, ok := info.Ctx.Value("user-type").(string); ok && userType == "vip" {
		return true
	}

	// 方法3: 从请求头中获取用户ID，根据ID判断是否为VIP
	if userID, ok := info.Ctx.Value("user-id").(string); ok {
		return p.isVIPUser(userID)
	}

	return false
}

// isVIPUser 根据用户ID判断是否为VIP用户
func (p *picker) isVIPUser(userID string) bool {
	if len(userID) > 0 && userID[0] == 'V' {
		return true
	}
	return false
}

// pickFromGroup 从指定分组中选择最佳连接
func (p *picker) pickFromGroup(conns []*conn) *conn {
	if len(conns) == 0 {
		return nil
	}

	// 平滑加权轮询算法
	var totalWeight int
	best := conns[0]
	for _, cc := range conns {
		totalWeight += cc.weight
		// 增加当前权重
		cc.currentWeight += cc.weight
		if cc.currentWeight > best.currentWeight {
			best = cc
		}
	}

	best.currentWeight -= totalWeight
	return best
}

type conn struct {
	group         string
	weight        int
	currentWeight int
	cc            balancer.SubConn
}
