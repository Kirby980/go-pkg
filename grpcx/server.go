package grpcx

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/Kirby980/study/webook/pkg/logger"
	"github.com/Kirby980/study/webook/pkg/netx"
	etcdv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc"
)

type Server struct {
	*grpc.Server
	Port      int
	EtcdAddrs []string
	Name      string
	key       string
	em        endpoints.Manager
	client    *etcdv3.Client
	kaCancel  context.CancelFunc
	L         logger.Logger
}

func (s *Server) Serve() error {
	l, err := net.Listen("tcp", ":"+strconv.Itoa(s.Port))
	if err != nil {
		panic(err)
	}
	err = s.register()
	if err != nil {
		return err
	}
	return s.Server.Serve(l)
}

func (s *Server) register() error {
	etcd, err := etcdv3.New(etcdv3.Config{
		Endpoints: s.EtcdAddrs,
	})
	if err != nil {
		return err
	}
	s.client = etcd
	em, err := endpoints.NewManager(etcd, "service/"+s.Name)
	if err != nil {
		return err
	}
	s.em = em
	addr := netx.GetOutboundIP() + ":" + strconv.Itoa(s.Port)
	key := "service/" + s.Name + "/" + addr
	s.key = key
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	leaseResp, err := etcd.Grant(ctx, 30)
	if err != nil {
		return err
	}
	err = em.AddEndpoint(ctx, key, endpoints.Endpoint{
		Addr: addr,
	}, etcdv3.WithLease(leaseResp.ID))
	if err != nil {
		return err
	}
	kaCtx, kaCancel := context.WithCancel(context.Background())
	ch, err := s.client.KeepAlive(kaCtx, leaseResp.ID)
	s.kaCancel = kaCancel
	if err != nil {
		return err
	}
	go func() {
		for kaResp := range ch {
			// 正常就是打印一下 DEBUG 日志啥的
			s.L.Debug(kaResp.String())
		}
	}()
	return nil
}

func (s *Server) Close() error {
	if s.kaCancel != nil {
		s.kaCancel()
	}
	if s.em != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := s.em.DeleteEndpoint(ctx, s.key)
		if err != nil {
			return err
		}
	}
	if s.client != nil {
		err := s.client.Close()
		if err != nil {
			return err
		}
	}
	s.Server.GracefulStop()
	return nil
}
