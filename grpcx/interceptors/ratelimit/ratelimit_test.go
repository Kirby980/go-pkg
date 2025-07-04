package ratelimit

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kirby980/go-pkg"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// 模拟用户服务请求和响应
type SelectRequest struct {
	Id int64
}

type SelectResponse struct {
	Id       int64
	Nickname string
}

// 模拟用户服务接口
type UserServiceServer interface {
	Select(context.Context, *SelectRequest) (*SelectResponse, error)
}

// 模拟用户服务实现
type MockUserService struct{}

func (s *MockUserService) Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error) {
	// 模拟处理时间
	time.Sleep(time.Millisecond * 100)
	return &SelectResponse{
		Id:       req.Id,
		Nickname: "test user",
	}, nil
}

// 模拟gRPC服务注册
func RegisterUserServiceServer(s *grpc.Server, srv UserServiceServer) {
	// 这里只是模拟，实际不会注册
}

// 模拟gRPC客户端
type UserServiceClient interface {
	Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error)
}

type mockUserServiceClient struct {
	conn *grpc.ClientConn
}

func (c *mockUserServiceClient) Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error) {
	// 模拟客户端调用
	return &SelectResponse{Id: req.Id, Nickname: "test user"}, nil
}

func NewUserServiceClient(conn *grpc.ClientConn) UserServiceClient {
	return &mockUserServiceClient{conn: conn}
}

// 测试计数器限流器
func TestCounterLimiter(t *testing.T) {
	// 创建限流器
	limiter := &CounterLimiter{
		cnt:       &atomic.Int32{},
		threshold: 5, // 最大并发5个请求
	}

	// 创建 gRPC 服务器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(limiter.NewServerInterceptor()),
	)
	RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080") // 随机端口
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := NewUserServiceClient(conn)

	// 测试正常请求（前5个应该成功）
	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := client.Select(ctx, &SelectRequest{Id: 123})
			if err != nil {
				if status.Code(err) == codes.ResourceExhausted {
					errorCount++
				}
			} else {
				successCount++
			}
		}()
	}

	wg.Wait()

	// 验证结果
	require.Equal(t, 5, successCount, "应该有5个请求成功")
	require.Equal(t, 5, errorCount, "应该有5个请求被限流")
}

// 测试滑动窗口限流器
func TestSlideWindowLimiter(t *testing.T) {
	// 创建限流器
	limiter := &SlideWindowLimiter{
		window:    time.Second * 2, // 2秒窗口
		queue:     pkg.Queue[time.Time]{},
		threshold: 3, // 2秒内最多3个请求
	}

	// 创建 gRPC 服务器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(limiter.NewServerInterceptor()),
	)
	RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := NewUserServiceClient(conn)

	// 测试快速请求
	successCount := 0
	errorCount := 0

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := client.Select(ctx, &SelectRequest{Id: 123})
		cancel()

		if err != nil {
			if status.Code(err) == codes.ResourceExhausted {
				errorCount++
			}
		} else {
			successCount++
		}
	}

	// 验证结果
	require.Equal(t, 3, successCount, "应该有3个请求成功")
	require.Equal(t, 2, errorCount, "应该有2个请求被限流")

	// 等待窗口过期
	time.Sleep(time.Second * 3)

	// 再次测试，应该又能成功
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = client.Select(ctx, &SelectRequest{Id: 123})
	cancel()
	require.NoError(t, err, "窗口过期后应该能成功请求")
}

// 测试令牌桶限流器
func TestTokenBucketLimiter(t *testing.T) {
	// 创建限流器
	limiter := NewTokenBucketLimiter(time.Millisecond*500, 2) // 每500ms一个令牌，容量2
	defer limiter.Close()

	// 创建 gRPC 服务器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(limiter.NewServerInterceptor()),
	)
	RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := NewUserServiceClient(conn)

	// 测试令牌桶
	successCount := 0
	timeoutCount := 0

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		_, err := client.Select(ctx, &SelectRequest{Id: 123})
		cancel()

		if err != nil {
			if status.Code(err) == codes.DeadlineExceeded {
				timeoutCount++
			}
		} else {
			successCount++
		}
	}

	// 验证结果
	require.Equal(t, 2, successCount, "应该有2个请求成功(桶容量为2)")
	require.Equal(t, 3, timeoutCount, "应该有3个请求超时")
}

func TestLeakBucketLimiter(t *testing.T) {
	limiter := NewLeakBucketLimiter(time.Millisecond * 500)
	defer limiter.Close()

	server := grpc.NewServer(
		grpc.UnaryInterceptor(limiter.NewServerInterceptor()),
	)
	RegisterUserServiceServer(server, &MockUserService{})

	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := NewUserServiceClient(conn)

	successCount := 0
	timeoutCount := 0

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		_, err := client.Select(ctx, &SelectRequest{Id: 123})
		cancel()

		if err != nil {
			if status.Code(err) == codes.DeadlineExceeded {
				timeoutCount++
			}
		} else {
			successCount++
		}
	}

	require.Equal(t, 2, successCount, "应该有2个请求成功(桶容量为2)")
	require.Equal(t, 3, timeoutCount, "应该有3个请求超时")
}
