// Job - обработка новых заказов
// Опрос Kafka -> создание транзакций с датой в будущем (14дней)
package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	db "github.com/glkeru/loyalty/points/internal/db"
	kafka "github.com/glkeru/loyalty/points/internal/external/kafka"
	interf "github.com/glkeru/loyalty/points/internal/interfaces"
	services "github.com/glkeru/loyalty/points/internal/services"
	"go.uber.org/zap"
)

// Job - Обработка заказов
func main() {
	// log
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// kafka
	reader, err := kafka.GetNewReader("orders")
	if err != nil {
		panic(err)
	}
	defer reader.CloseReader()

	// database
	var storage interf.PointsStorage
	dt, err := db.NewPointsDB(logger)
	if err != nil {
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
	semenv := os.Getenv("POINTS_ORDERS_COUNT")
	if semenv == "" {
		semcount = 5
	} else {
		semcount, err = strconv.Atoi(semenv)
		if err != nil {
			semcount = 5
		}
	}
	if semcount == 0 {
		semcount = 1
	}

	wg := &sync.WaitGroup{}
	semaphore := make(chan struct{}, semcount)

loop:
	for {
		select {
		case <-interrupt:
			cancel()
			break loop
		case <-ctx.Done():
			break loop
		default:

			order, err := reader.GetNewMessage(ctx)
			if err != nil {
				logger.Error(err.Error())
				return
			}

			semaphore <- struct{}{}
			wg.Add(1)
			go func(order string) {
				defer wg.Done()
				defer func() { <-semaphore }()
				err = serv.OrderCalculate(ctx, order)
				if err != nil {
					logger.Error(err.Error())
					return
				}
			}(order)
		}
	}
	wg.Wait()
}
