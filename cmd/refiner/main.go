package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"context-refiner/internal/bootstrap"

	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime, err := bootstrap.LoadRuntime(ctx, "config/service.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := runtime.RedisRepository.Close(); closeErr != nil {
			log.Printf("close redis store failed: %v", closeErr)
		}
	}()

	lis, err := net.Listen("tcp", runtime.Cfg.GRPC.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		<-ctx.Done()
		runtime.GRPCServer.GracefulStop()
	}()

	bootstrap.StartSummaryWorker(ctx, runtime)

	log.Printf("refiner gRPC server listening on %s", runtime.Cfg.GRPC.ListenAddr)
	if err := runtime.GRPCServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Fatal(err)
	}
}
