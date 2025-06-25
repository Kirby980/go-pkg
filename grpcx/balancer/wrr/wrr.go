package wrr

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
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

	conns := make([]*conn, 0, len(info.ReadySCs))
	for sc, sci := range info.ReadySCs {
		cc := &conn{
			cc: sc,
		}
		md, ok := sci.Address.Metadata.(map[string]any)
		if ok {
			weightVal := md["weight"]
			weight, _ := weightVal.(float64)
			cc.weight = int(weight)
		}
		if cc.weight == 0 {
			cc.weight = 10
		}
		cc.currentWeight = cc.weight
		conns = append(conns, cc)
	}

	return &picker{
		conns: conns,
	}
}

type picker struct {
	conns []*conn
	mutex sync.Mutex
}

func (p *picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.conns) == 0 {
		// 没有候选节点
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// 平滑加权轮询算法
	var totalWeight int
	best := p.conns[0]
	for _, cc := range p.conns {
		totalWeight += cc.weight
		// 增加当前权重
		cc.currentWeight += cc.weight
		if cc.currentWeight > best.currentWeight {
			best = cc
		}
	}

	best.currentWeight -= totalWeight
	return balancer.PickResult{
		SubConn: best.cc,
		Done: func(di balancer.DoneInfo) {
			// 这个地方是用来调整动态算法的，根据结果调整权重
		},
	}, nil
}

type conn struct {
	weight        int
	currentWeight int
	cc            balancer.SubConn
}
