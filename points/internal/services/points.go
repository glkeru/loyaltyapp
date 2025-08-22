package points

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	external "github.com/glkeru/loyalty/points/internal/external/engine"
	interf "github.com/glkeru/loyalty/points/internal/interfaces"
	model "github.com/glkeru/loyalty/points/internal/models"
	"go.uber.org/zap"
)

type PointsService struct {
	logger *zap.Logger
	db     interf.PointsStorage
	cache  interf.CacheStorage
}

func NewPointService(logger *zap.Logger, db interf.PointsStorage, cache interf.CacheStorage) (service *PointsService) {
	return &PointsService{logger, db, cache}
}

// Расчет баллов по заказу
func (p *PointsService) OrderCalculate(ctx context.Context, order string) error {
	// рассчет баллов
	points, err := external.CalculateOrder(ctx, order)
	if err != nil {
		return err
	}
	// получить userId и orderId из переданного заказа
	userId, orderId, err := GetUserAndOrder(order)
	if err != nil {
		return err
	}
	// сохранить транзакцию начисления
	err = p.TnxOrderAccruelCreate(ctx, userId, float64(points), orderId)
	if err != nil {
		return err
	}
	return nil
}

// Обработка возврата
func (p *PointsService) ReturnProcess(ctx context.Context, order string) error {
	// получить orderId из переданного заказа
	_, orderId, err := GetUserAndOrder(order)
	if err != nil {
		return err
	}

	p.logger.Info("return",
		zap.String("id", order))
	// удалить транзакцию начисления
	err = p.TnxDelete(ctx, orderId)
	if err != nil {
		return err
	}
	return nil
}

type OrderStruct struct {
	OrderId string `json:"orderId"`
	UserId  string `json:"userId"`
}

func GetUserAndOrder(orderJson string) (userId string, orderId string, err error) {
	orderParams := &OrderStruct{}
	err = json.Unmarshal([]byte(orderJson), orderParams)
	if err != nil {
		return
	}

	userId = orderParams.UserId
	if userId == "" {
		return "", "", fmt.Errorf("Invalid order: userId field is required")
	}

	orderId = orderParams.OrderId
	if orderId == "" {
		return "", "", fmt.Errorf("Invalid order: orderId field is required")
	}
	return
}

// запуск активации баллов на дату
// TODO: вернуть пользователей, чтобы инвлидировать кэш балансов
func (p *PointsService) CommitOnDate(ctx context.Context) error {
	date := time.Now()
	//TODO: инвалидировать кэш балансов, которые поменялись
	err := p.db.TnxCommitOnDate(ctx, date)
	if err != nil {
		return err
	}
	return nil
}

// создание транзакции начисления
func (p *PointsService) TnxOrderAccruelCreate(ctx context.Context, userId string, points float64, orderId string) error {
	tnx := model.PointTransaction{}
	tnx.Points = points

	// TODO DEFAULT
	var dayscount int
	var err error
	daysenv := os.Getenv("POINTS_DAYS_COUNT")
	if daysenv == "" {
		dayscount = 0
	} else {
		dayscount, err = strconv.Atoi(daysenv)
		if err != nil {
			dayscount = 0
		}
	}
	tnx.CommitDate = time.Now().Add(time.Duration(dayscount) * 24 * time.Hour)
	tnx.TypeTnx = model.ACCRUEL
	tnx.OrderID = orderId
	account, err := p.db.GetUserUUID(ctx, userId)
	if err != nil {
		return err
	}
	tnx.PointAccount = account
	err = p.db.TnxCreate(ctx, tnx)
	if err != nil {
		return err
	}
	return nil
}

// удаление транзакции
func (p *PointsService) TnxDelete(ctx context.Context, orderId string) error {
	err := p.db.TnxDelete(ctx, orderId)
	if err != nil {
		return err
	}
	return nil
}

// создание транзакции списания
type RedeemStruct struct {
	UserId   string  `json:"userId"`
	Points   float64 `json:"points"`
	RedeemId string  `json:"redeemId"`
}

// cписание
func (p *PointsService) Redeem(ctx context.Context, redeemJson string) (redeemId string, err error) {
	redeem := &RedeemStruct{}
	err = json.Unmarshal([]byte(redeemJson), redeem)
	if err != nil {
		return "", err
	}
	err = p.TnxRedeemCreate(ctx, redeem.UserId, redeem.Points, redeem.RedeemId)
	if err != nil {
		return redeem.RedeemId, err
	}
	return redeem.RedeemId, nil
}

// создание транзакции списания
func (p *PointsService) TnxRedeemCreate(ctx context.Context, userId string, points float64, redeemId string) error {
	err := p.db.Redeem(ctx, userId, points, redeemId)
	if err != nil {
		return err
	}
	if p.cache != nil {
		err = p.InvalidateBalance(ctx, userId)
		if err != nil {
			p.logger.Error(err.Error())
		}
	}

	return nil
}

// перевод баллов
func (p *PointsService) Transfer(ctx context.Context, userfrom string, userto string, points float64, transferId string) error {
	err := p.db.Transfer(ctx, userfrom, userto, points, transferId)
	if err != nil {
		return err
	}

	if p.cache != nil {
		err = p.InvalidateBalance(ctx, userfrom)
		if err != nil {
			p.logger.Error(err.Error())
		}
		err = p.InvalidateBalance(ctx, userto)
		if err != nil {
			p.logger.Error(err.Error())
		}
	}
	return nil
}

// баланс
func (p *PointsService) GetBalance(ctx context.Context, user string) (points float64, err error) {
	// cache
	if p.cache != nil {
		points, err = p.cache.GetBalance(ctx, user)
		if err != nil {
			// database
			points, err = p.db.GetBalance(ctx, user)
			if err != nil {
				return 0, err
			}
			_ = p.cache.SetBalance(ctx, user, points)
		}
	} else {
		points, err = p.db.GetBalance(ctx, user)
		if err != nil {
			return 0, err
		}
	}
	return
}

// инвалидировать кэш баланса
func (p *PointsService) InvalidateBalance(ctx context.Context, user string) error {
	if p.cache != nil {
		err := p.cache.InvalidateBalance(ctx, user)
		return err
	}
	return nil
}

// транзакции
func (p *PointsService) GetTnx(ctx context.Context, user string, from time.Time, to time.Time) (tnxs []model.PointTransaction, err error) {
	tnxs, err = p.db.GetTnx(ctx, user, from, to)
	if err != nil {
		return nil, err
	}
	return tnxs, nil

}

func (p *PointsService) Log(err error) {
	p.logger.Error(err.Error())
}
