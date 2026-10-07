package main

import (
	"log"
	"net"
	"sync"
	"time"

	proto "gacha-simulator/proto"

	"google.golang.org/grpc"
)

type GRPCConfigServer struct {
	proto.UnimplementedConfigServiceServer
	mu          sync.Mutex
	subscribers map[chan *proto.PoolConfigMessage]struct{}
}

var DefaultGRPCServer = &GRPCConfigServer{
	subscribers: make(map[chan *proto.PoolConfigMessage]struct{}),
}

func (s *GRPCConfigServer) SubscribeConfigUpdates(req *proto.EmptyRequest, stream proto.ConfigService_SubscribeConfigUpdatesServer) error {
	ch := make(chan *proto.PoolConfigMessage, 10)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		s.mu.Unlock()
	}()

	cfg := GetCurrentPoolConfig()
	initMsg := &proto.PoolConfigMessage{
		BaseRateS:     cfg.BaseRateS,
		BaseRateA:     cfg.BaseRateA,
		BaseRateB:     cfg.BaseRateB,
		SoftPityStart: int32(cfg.SoftPityStart),
		SoftPityInc:   cfg.SoftPityInc,
		HardPityS:     int32(cfg.HardPityS),
		HardPityA:     int32(cfg.HardPityA),
		MaxLimitedS:   int32(cfg.MaxLimitedS),
		UpdatedAt:     time.Now().Unix(),
	}

	if err := stream.Send(initMsg); err != nil {
		return err
	}

	ctx := stream.Context()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func BroadcastConfigToGRPC(cfg *PoolConfig) {
	if cfg == nil {
		return
	}
	msg := &proto.PoolConfigMessage{
		BaseRateS:     cfg.BaseRateS,
		BaseRateA:     cfg.BaseRateA,
		BaseRateB:     cfg.BaseRateB,
		SoftPityStart: int32(cfg.SoftPityStart),
		SoftPityInc:   cfg.SoftPityInc,
		HardPityS:     int32(cfg.HardPityS),
		HardPityA:     int32(cfg.HardPityA),
		MaxLimitedS:   int32(cfg.MaxLimitedS),
		UpdatedAt:     time.Now().Unix(),
	}

	DefaultGRPCServer.mu.Lock()
	defer DefaultGRPCServer.mu.Unlock()

	for ch := range DefaultGRPCServer.subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
}

var grpcServerInstance *grpc.Server
var grpcListener net.Listener
var grpcMu sync.Mutex

func StartGRPCServer(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[GRPC] Failed to listen on %s: %v", addr, err)
		return err
	}

	s := grpc.NewServer()
	proto.RegisterConfigServiceServer(s, DefaultGRPCServer)

	grpcMu.Lock()
	grpcServerInstance = s
	grpcListener = lis
	grpcMu.Unlock()

	log.Printf("[GRPC] ConfigService listening on %s", addr)
	return s.Serve(lis)
}

func StopGRPCServer() {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	if grpcServerInstance != nil {
		grpcServerInstance.GracefulStop()
		grpcServerInstance = nil
	}
}
