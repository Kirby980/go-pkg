package wrr

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// 模拟用户服务请求和响应结构体
type SelectRequest struct {
	Id int64
}

type SelectResponse struct {
	Id   int64
	Name string
}

// 模拟用户服务客户端接口
type UserServiceClient interface {
	Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error)
}

// 模拟用户服务客户端实现
type mockUserServiceClient struct{}

func (m *mockUserServiceClient) Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error) {
	return &SelectResponse{Id: req.Id, Name: "mock_user"}, nil
}

// 示例：VIP分组负载均衡
func ExampleVIPBalancer() {
	// 1. 创建 gRPC 连接，使用自定义负载均衡器
	cc, err := grpc.Dial("localhost:12379",
		grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"smooth_weight_round_robin": {}}]
		}`),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()

	client := &mockUserServiceClient{}

	// 2. VIP用户请求 - 方法1: 通过VIP标识
	ctx1 := metadata.AppendToOutgoingContext(context.Background(), "vip", "true")
	resp1, err := client.Select(ctx1, &SelectRequest{Id: 123})
	if err != nil {
		log.Printf("VIP请求失败: %v", err)
	} else {
		log.Printf("VIP响应: %+v", resp1)
	}

	// 3. VIP用户请求 - 方法2: 通过用户类型
	ctx2 := metadata.AppendToOutgoingContext(context.Background(), "user-type", "vip")
	resp2, err := client.Select(ctx2, &SelectRequest{Id: 456})
	if err != nil {
		log.Printf("VIP请求失败: %v", err)
	} else {
		log.Printf("VIP响应: %+v", resp2)
	}

	// 4. VIP用户请求 - 方法3: 通过用户ID
	ctx3 := metadata.AppendToOutgoingContext(context.Background(), "user-id", "V12345")
	resp3, err := client.Select(ctx3, &SelectRequest{Id: 789})
	if err != nil {
		log.Printf("VIP请求失败: %v", err)
	} else {
		log.Printf("VIP响应: %+v", resp3)
	}

	// 5. 普通用户请求
	resp4, err := client.Select(context.Background(), &SelectRequest{Id: 999})
	if err != nil {
		log.Printf("普通请求失败: %v", err)
	} else {
		log.Printf("普通响应: %+v", resp4)
	}
}

// 测试VIP分组功能
func TestVIPBalancer(t *testing.T) {
	// 模拟VIP判断逻辑
	tests := []struct {
		name     string
		userID   string
		expected bool
	}{
		{"VIP用户ID以V开头", "V12345", true},
		{"普通用户ID", "U12345", false},
		{"空用户ID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVIPUser(tt.userID)
			require.Equal(t, tt.expected, result)
		})
	}
}

// 服务端注册示例
func ExampleVIPServerRegistration() {
	// VIP服务端节点注册
	vipServerInfo := map[string]interface{}{
		"weight": 20,        // VIP节点权重更高
		"group":  "vip",     // VIP分组
		"region": "us-east", // 其他元数据
	}

	// 普通服务端节点注册
	normalServerInfo := map[string]interface{}{
		"weight": 10,        // 普通节点权重较低
		"group":  "normal",  // 普通分组
		"region": "us-east", // 其他元数据
	}

	fmt.Printf("注册VIP节点，分组: %s, 权重: %d\n",
		vipServerInfo["group"], vipServerInfo["weight"])
	fmt.Printf("注册普通节点，分组: %s, 权重: %d\n",
		normalServerInfo["group"], normalServerInfo["weight"])
}

// VIP分组策略说明
func ExampleVIPStrategy() {
	fmt.Println("VIP分组策略:")
	fmt.Println("1. VIP用户优先使用VIP分组节点")
	fmt.Println("2. VIP分组无可用节点时,降级到普通分组")
	fmt.Println("3. 普通用户使用普通分组节点")
	fmt.Println("4. 普通分组无可用节点时,从所有分组中选择")
	fmt.Println("")
	fmt.Println("VIP识别方式:")
	fmt.Println("- 请求头中的 vip=true")
	fmt.Println("- 请求头中的 user-type=vip")
	fmt.Println("- 请求头中的 user-id (以V开头)")
}

// 模拟VIP用户判断函数（用于测试）
func isVIPUser(userID string) bool {
	if len(userID) > 0 && userID[0] == 'V' {
		return true
	}
	return false
}
