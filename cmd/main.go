package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime, err := loadRuntime(ctx, "config/service.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := runtime.pageStore.Close(); closeErr != nil {
			log.Printf("close redis store failed: %v", closeErr)
		}
	}()

	lis, err := net.Listen("tcp", runtime.cfg.GRPC.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		<-ctx.Done()
		runtime.grpcServer.GracefulStop()
	}()

	startSummaryWorker(ctx, runtime)

	log.Printf("refiner gRPC server listening on %s", runtime.cfg.GRPC.ListenAddr)
	if err := runtime.grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Fatal(err)
	}
}
