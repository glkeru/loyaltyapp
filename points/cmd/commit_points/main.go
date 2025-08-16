// Job - начисление отложенных баллов (разметка транзакций и изменение баланса)
// Если дата начисления наступила - транзакция размечается флагом, баллы добавляются на баланс
package main

import (
	"context"

	"go.uber.org/zap"

	db "github.com/glkeru/loyalty/points/internal/db"
	interf "github.com/glkeru/loyalty/points/internal/interfaces"
	services "github.com/glkeru/loyalty/points/internal/services"
)

func main() {
	// log
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

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

	serv := services.NewPointService(logger, storage, redis)
	err = serv.CommitOnDate(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info("Job Tnx commit on date is finished")
}
