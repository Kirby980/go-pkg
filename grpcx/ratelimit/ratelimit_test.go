package ratelimit

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	userv1 "github.com/Kirby980/study/webook/api/proto/gen/user/v1"
	"github.com/Kirby980/study/webook/pkg"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// 模拟用户服务
type MockUserService struct {
	userv1.UnimplementedUserServiceServer
}

func (s *MockUserService) Select(ctx context.Context, req *userv1.SelectRequest) (*userv1.SelectResponse, error) {
	// 模拟处理时间
	time.Sleep(time.Millisecond * 100)
	return &userv1.SelectResponse{
		User: &userv1.User{
			Id:       req.Id,
			Nickname: "test user",
		},
	}, nil
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
	userv1.RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080") // 随机端口
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := userv1.NewUserServiceClient(conn)

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

			_, err := client.Select(ctx, &userv1.SelectRequest{Id: 123})
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
	userv1.RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := userv1.NewUserServiceClient(conn)

	// 测试快速请求
	successCount := 0
	errorCount := 0

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := client.Select(ctx, &userv1.SelectRequest{Id: 123})
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
	_, err = client.Select(ctx, &userv1.SelectRequest{Id: 123})
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
	userv1.RegisterUserServiceServer(server, &MockUserService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	go server.Serve(lis)
	defer server.Stop()

	// 创建客户端
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := userv1.NewUserServiceClient(conn)

	// 测试令牌桶
	successCount := 0
	timeoutCount := 0

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		_, err := client.Select(ctx, &userv1.SelectRequest{Id: 123})
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
	require.Equal(t, 2, successCount, "应该有2个请求成功（桶容量）")
	require.Equal(t, 3, timeoutCount, "应该有3个请求超时")
}
