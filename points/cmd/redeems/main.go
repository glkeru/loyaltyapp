// Job - Обработка списаний баллов
package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	db "github.com/glkeru/loyalty/points/internal/db"
	rabbit "github.com/glkeru/loyalty/points/internal/external/rabbitmq"
	interf "github.com/glkeru/loyalty/points/internal/interfaces"
	services "github.com/glkeru/loyalty/points/internal/services"
	"go.uber.org/zap"
)

func main() {
	// log
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// rabbitmq
	reader, err := rabbit.NewRabbitConsumer()
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}
	defer reader.Close()

	// database
	var storage interf.PointsStorage
	dt, err := db.NewPointsDB(logger)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}
	storage = dt

	// cache
	var redis interf.CacheStorage
	redis, err = db.NewCacheService()
	if err != nil {
		logger.Error(err.Error())
	}

	// services
	serv := services.NewPointService(logger, storage, redis)

	// start
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: default
	var semcount int
	semenv := os.Getenv("POINTS_REDEEM_COUNT")
	if semenv == "" {
		semcount = 5
	} else {
		semcount, err = strconv.Atoi(semenv)
		if err != nil {
			semcount = 5
		}
	}

	// os signals
	go func() {
		<-interrupt
		cancel()
	}()

	// workers
	wg := &sync.WaitGroup{}
	wg.Add(semcount)
	for i := 0; i < semcount; i++ {
		go worker(ctx, serv, wg, logger, reader)
	}
	wg.Wait()
}

// worker for rabbitmq messages
func worker(ctx context.Context, serv *services.PointsService, wg *sync.WaitGroup, logger *zap.Logger, reader *rabbit.RabbitConsumer) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := <-reader.Msg
			if ok != true {
				return
			}
			redeemId, err := serv.Redeem(ctx, string(msg.Body))
			if err != nil {
				logger.Error(err.Error())
				if redeemId != "" {
					_ = reader.Processed(ctx, redeemId, false)
				}
				continue
			}
			err = reader.Processed(ctx, redeemId, true)
			if err != nil {
				logger.Error(err.Error())
				continue
			}

		}
	}
}
