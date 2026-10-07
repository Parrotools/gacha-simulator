package main

import (
	"context"
	"io"
	"log"
	"time"

	proto "gacha-simulator/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func StartGRPCConfigClient(addr string) {
	StartGRPCConfigClientWithContext(context.Background(), addr)
}

func StartGRPCConfigClientWithContext(ctx context.Context, addr string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				// Avoid noisy logs if context is canceled
				if ctx.Err() != nil {
					return
				}
				time.Sleep(1 * time.Second)
				continue
			}

			client := proto.NewConfigServiceClient(conn)
			stream, err := client.SubscribeConfigUpdates(ctx, &proto.EmptyRequest{})
			if err != nil {
				conn.Close()
				if ctx.Err() != nil {
					return
				}
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("[GameServer] gRPC stream connected to %s", addr)
			for {
				msg, err := stream.Recv()
				if err == io.EOF || ctx.Err() != nil {
					break
				}
				if err != nil {
					log.Printf("[GameServer] gRPC stream error: %v", err)
					break
				}

				newCfg := PoolConfig{
					BaseRateS:     msg.BaseRateS,
					BaseRateA:     msg.BaseRateA,
					BaseRateB:     msg.BaseRateB,
					SoftPityStart: int(msg.SoftPityStart),
					SoftPityInc:   msg.SoftPityInc,
					HardPityS:     int(msg.HardPityS),
					HardPityA:     int(msg.HardPityA),
					MaxLimitedS:   int(msg.MaxLimitedS),
				}
				GlobalConfigAtomic.Store(&newCfg)
				log.Printf("[GameServer] Hot-reloaded pool config via gRPC stream: S rate=%.4f, HardPityS=%d", newCfg.BaseRateS, newCfg.HardPityS)
			}

			conn.Close()
			time.Sleep(1 * time.Second)
		}
	}()
}
