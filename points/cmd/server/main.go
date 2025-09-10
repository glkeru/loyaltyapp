// gRPC server - обработка запросов на получение баланса и истории транзакций
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	serv "github.com/glkeru/loyalty/points/internal/api/grpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {

	// config
	port := os.Getenv("POINTS_GRPC_PORT")
	if port == "" {
		panic("env POINTS_GRPC_PORT is not set")
	}
	// log
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		panic(err)
	}

	interrrupt := make(chan os.Signal, 1)
	signal.Notify(interrrupt, os.Interrupt, syscall.SIGTERM)

	grpcServer := grpc.NewServer()
	serv.RegisterGetPointsServer(grpcServer, serv.NewPointsService(logger))

	go func() {
		err := grpcServer.Serve(lis)
		if err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	<-interrrupt
	grpcServer.GracefulStop()
}
