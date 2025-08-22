package grpc

import (
	context "context"
	"errors"
	"time"

	db "github.com/glkeru/loyalty/points/internal/db"
	interf "github.com/glkeru/loyalty/points/internal/interfaces"
	model "github.com/glkeru/loyalty/points/internal/models"
	services "github.com/glkeru/loyalty/points/internal/services"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"

	"go.uber.org/zap"
)

type PointsService struct {
	service *services.PointsService
	UnimplementedGetPointsServer
}

func NewPointsService() *PointsService {
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
		redis = nil
	}
	serv := services.NewPointService(logger, storage, redis)
	return &PointsService{serv, UnimplementedGetPointsServer{}}
}

// Баланс
func (p *PointsService) GetBalance(ctx context.Context, in *BalanceRequest) (*BalanceResponse, error) {
	points, err := p.service.GetBalance(ctx, in.User)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		p.service.Log(err)
		return nil, err
	}
	return &BalanceResponse{
		Points: points,
	}, nil
}

// История транзакций
func (p *PointsService) GetTnx(ctx context.Context, in *TnxRequest) (*TnxResponse, error) {
	user := in.User
	from, err := time.Parse("2006-01-02 15:04:05", in.Datefrom+" 00:00:00")
	if err != nil {
		p.service.Log(err)
		return nil, err
	}
	to, err := time.Parse("2006-01-02 15:04:05", in.Dateto+" 23:59:59")
	if err != nil {
		p.service.Log(err)
		return nil, err
	}
	// получить транзакции
	tnxs, err := p.service.GetTnx(ctx, user, from, to)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		p.service.Log(err)
		return nil, err
	}
	// сформировать ответ
	count := len(tnxs)
	resp := make([]*TnxMessage, count)
	for i, v := range tnxs {
		resp[i] = &TnxMessage{
			UUID:       v.UUID.String(),
			Points:     v.Points,
			CommitDate: v.CommitDate.String(),
			Commit:     v.Commit,
			TypeTnx:    int32(v.TypeTnx),
			Order:      v.OrderID,
			Transfer:   v.TransferID,
			Redeem:     v.RedeemID,
		}
	}
	return &TnxResponse{Tnx: resp}, nil
}
