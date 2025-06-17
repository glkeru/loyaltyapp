package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/glkeru/loyalty/engine/internal/api"
	db "github.com/glkeru/loyalty/engine/internal/db"
	engine "github.com/glkeru/loyalty/engine/internal/interfaces"
	"go.uber.org/zap"
)

func main() {
	// log
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// config
	port := os.Getenv("ENGINE_PORT")
	if port == "" {
		panic("env ENGINE_PORT is not set")
	}

	// database
	var storage engine.RuleStorage
	dt, err := db.NewRulesDB()
	if err != nil {
		panic(err)
	}
	storage = dt

	// api handlers
	r := api.NewHandler(&storage, logger)
	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	go srv.ListenAndServe()

	// shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(timeout)
	if err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
