package points

import (
	"time"

	"github.com/google/uuid"
)

// Счет баллов
type PointAccount struct {
	UUID    uuid.UUID
	Balance float64 // баланс
	User    string  // ID пользователя
}

const (
	ACCRUEL = 0
	REDEEM  = 1
)

// Транзакции
type PointTransaction struct {
	UUID         uuid.UUID
	PointAccount uuid.UUID // UUID счета
	Points       float64   // кол-во баллов
	CommitDate   time.Time // дата/время транзакции / дата в будущем, в которую начислить баллы
	Commit       bool      // транзакция обработана
	TypeTnx      int       // тип операции
	OrderID      string    // ID заказа
	TransferID   string    // ID операции перевода баллов
	RedeemID     string    // ID операции списания баллов
}
