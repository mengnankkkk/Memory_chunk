package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"context-refiner/internal/bootstrap"

	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config/service.yaml", "path to app config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime, err := bootstrap.LoadRuntime(ctx, *configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := runtime.TraceShutdown(shutdownCtx); closeErr != nil {
			log.Printf("shutdown tracing provider failed: %v", closeErr)
		}
	}()
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
	bootstrap.StartMetricsServer(ctx, runtime)
	bootstrap.StartDashboardServer(ctx, runtime)

	log.Printf("refiner gRPC server listening on %s", runtime.Cfg.GRPC.ListenAddr)
	if err := runtime.GRPCServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Fatal(err)
	}
}
