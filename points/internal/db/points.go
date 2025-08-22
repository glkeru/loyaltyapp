package points

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	model "github.com/glkeru/loyalty/points/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PointsDB struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPointsDB(logger *zap.Logger) (db *PointsDB, err error) {
	// config
	purl := os.Getenv("POINTS_DB")
	if purl == "" {
		return nil, fmt.Errorf("env POINTS_DB is not set")
	}
	port := os.Getenv("POINTS_DB_PORT")
	if port == "" {
		return nil, fmt.Errorf("env POINTS_DB_PORT is not set")
	}
	user := os.Getenv("POINTS_DB_USER")
	if user == "" {
		return nil, fmt.Errorf("env POINTS_DB_USER is not set")
	}
	password := os.Getenv("POINTS_DB_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("env POINTS_DB_PASSWORD is not set")
	}
	database := os.Getenv("POINTS_DB_BASE")
	if database == "" {
		return nil, fmt.Errorf("env POINTS_DB_BASE is not set")
	}
	dsn := "postgres://" + user + ":" + password + "@" + purl + ":" + port + "/" + database

	pool, err := pgxpool.New(context.Background(), dsn)
	return &PointsDB{pool, logger}, err
}

// Создание транзакции начисления с датой в будущем
func (p *PointsDB) TnxCreate(ctx context.Context, tnx model.PointTransaction) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tnx.UUID = uuid.New()

	sql, args, err := sq.Insert("tnx").
		Columns("id", "pointaccount", "points", "commitdate", "typetnx", "orderid", "transferid", "redeemid").
		Values(tnx.UUID, tnx.PointAccount, tnx.Points, tnx.CommitDate, model.ACCRUEL, tnx.OrderID, tnx.TransferID, tnx.RedeemID).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		p.logger.Error("SQL error",
			zap.Error(err),
			zap.String("query", sql),
			zap.Any("args", args),
		)
		return err
	}

	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		p.logger.Error("SQL error",
			zap.Error(err),
			zap.String("query", sql),
			zap.Any("args", args),
		)
		return err
	}
	return nil
}

// Создание пользователя
func (p *PointsDB) UserCreate(ctx context.Context, userid string) (useruuid uuid.UUID, err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer conn.Release()

	useruuid = uuid.New()

	sql, args, err := sq.Insert("accounts").
		Columns("uuid", "userid", "balance").
		PlaceholderFormat(sq.Dollar).
		Values(useruuid, userid, 0).
		ToSql()
	if err != nil {
		p.logger.Error("SQL error",
			zap.Error(err),
			zap.String("query", sql),
			zap.Any("args", args),
		)
		return uuid.Nil, err
	}

	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		p.logger.Error("SQL error",
			zap.Error(err),
			zap.String("query", sql),
			zap.Any("args", args),
		)
		return uuid.Nil, err
	}
	return useruuid, nil
}

// Удаление транзакции (возвраты)
func (p *PointsDB) TnxDelete(ctx context.Context, orderId string) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	sql, args, err := sq.Delete("tnx").
		Where(sq.Eq{"orderid": orderId}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	return nil
}

// Зачисление баллов - обработка транзакции с наступившей датой
func (p *PointsDB) TnxCommitOnDate(ctx context.Context, date time.Time) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		p.logger.Error("Get connection error", zap.Error(err), zap.String("service", "TnxCommitOnDate"))
		return err
	}
	defer conn.Release()

	// получить все транзакции для коммита - сгруппировать по счетам
	sql, args, err := sq.Select("pointaccount", "SUM(points) as points").
		From("tnx").
		Where(sq.Eq{"commit": false}).
		Where(sq.LtOrEq{"commitdate": date}).
		GroupBy("pointaccount").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		p.logger.Error("SQL error",
			zap.Error(err),
			zap.String("query", sql),
			zap.Any("args", args),
		)
		return err
	}

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		p.logger.Error("Query get tnx error", zap.Error(err), zap.String("service", "TnxCommitOnDate"))
		return err
	}
	defer rows.Close()

	// TODO DEFAULT
	var semcount int
	semenv := os.Getenv("POINTS_BALANCE_COUNT")
	if semenv == "" {
		semcount = 3
	} else {
		semcount, err = strconv.Atoi(semenv)
		if err != nil {
			semcount = 3
		}
	}
	if semcount == 0 {
		semcount = 1
	}

	// семафор
	semch := make(chan struct{}, semcount)
	wg := &sync.WaitGroup{}

	// обработка счетов
	for rows.Next() {
		var balance uuid.UUID
		var points float64
		rows.Scan(&balance, &points)

		semch <- struct{}{}
		wg.Add(1)
		go func(balance uuid.UUID, points float64) {
			defer func() {
				wg.Done()
				<-semch
			}()

			conn, err := p.pool.Acquire(ctx)
			if err != nil {
				p.logger.Error("Get connection error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				return
			}
			defer conn.Release()

			tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				p.logger.Error("Begin tx error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				return
			}
			var erroroccured bool
			defer func() {
				if erroroccured == true {
					tx.Rollback(ctx)
				}
			}()

			// блокируем строку с балансом
			var currentb float64
			row := tx.QueryRow(ctx, "SELECT balance from ACCOUNTS where uuid = $1 FOR UPDATE", balance)
			err = row.Scan(&currentb)
			if err != nil {
				p.logger.Error("Block balance error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				erroroccured = true
				return
			}

			currentb += points

			// обновляем баланс
			sql, args, err := sq.Update("accounts").
				Set("balance", currentb).
				Where(sq.Eq{"uuid": balance}).
				PlaceholderFormat(sq.Dollar).
				ToSql()
			_, err = tx.Exec(ctx, sql, args...)
			if err != nil {
				p.logger.Error("Update balance error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				erroroccured = true
				return
			}

			// ставим флаг на транзакции
			sql, args, err = sq.Update("tnx").
				Set("commit", true).
				Where(sq.Eq{"pointaccount": balance}).
				Where(sq.Eq{"commit": false}).
				Where(sq.LtOrEq{"commitdate": date}).
				PlaceholderFormat(sq.Dollar).
				ToSql()
			_, err = tx.Exec(ctx, sql, args...)
			if err != nil {
				p.logger.Error("Commit tnx error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				erroroccured = true
				return
			}
			err = tx.Commit(ctx)
			if err != nil {
				p.logger.Error("Commit error",
					zap.Error(err),
					zap.String("service", "TnxCommitOnDate"),
					zap.String("balance", balance.String()))
				erroroccured = true
				return
			}

		}(balance, points)

	}
	wg.Wait()
	return nil
}

// Списание
func (p *PointsDB) Redeem(ctx context.Context, user string, points float64, redeemId string) (err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// проверить и заблокировать баланс
	var currentb float64
	var account uuid.UUID
	var pguuid pgtype.UUID
	row := tx.QueryRow(ctx, "SELECT uuid, balance from ACCOUNTS where userid = $1 FOR UPDATE", user)
	if err != nil {
		return err
	}
	err = row.Scan(&pguuid, &currentb)
	if err != nil {
		return err
	}
	account, _ = uuid.FromBytes(pguuid.Bytes[:])
	if currentb < points {
		return fmt.Errorf("Not enough points")
	}
	currentb -= points
	// обновляем баланс
	sql, args, err := sq.Update("accounts").
		Set("balance", currentb).
		Where(sq.Eq{"userid": user}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	// добавить транзакцию списания
	sql, args, err = sq.Insert("tnx").
		Columns("id", "pointaccount", "points", "commitdate", "typetnx", "redeemid", "commit").
		Values(uuid.New(), account, points, time.Now(), model.REDEEM, redeemId, true).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	tx.Commit(ctx)
	return nil
	// обновить баланс

}

// Перевод баллов
func (p *PointsDB) Transfer(ctx context.Context, userfrom string, userto string, points float64, transferId string) (err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// проверить и заблокировать баланс
	var currentb float64
	var account uuid.UUID
	var pguuid pgtype.UUID
	row := tx.QueryRow(ctx, "SELECT uuid, balance from ACCOUNTS where userid = $1 FOR UPDATE", userfrom)
	if err != nil {
		return err
	}
	err = row.Scan(&pguuid, &currentb)
	if err != nil {
		return err
	}
	account, _ = uuid.FromBytes(pguuid.Bytes[:])
	if currentb < points {
		return fmt.Errorf("Not enough points")
	}
	currentb -= points
	// обновляем баланс
	sql, args, err := sq.Update("accounts").
		Set("balance", currentb).
		Where(sq.Eq{"userid": userfrom}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	// добавить транзакцию списания
	sql, args, err = sq.Insert("tnx").
		Columns("id", "pointaccount", "points", "commitdate", "typetnx", "transferid").
		Values(uuid.New(), account, points, time.Now(), model.REDEEM, transferId).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	// user 2
	row = tx.QueryRow(ctx, "SELECT uuid, balance from ACCOUNTS where userid = $1 FOR UPDATE", userto)
	if err != nil {
		return err
	}

	err = row.Scan(&pguuid, &currentb)
	if err != nil {
		return err
	}
	account, _ = uuid.FromBytes(pguuid.Bytes[:])
	currentb += points
	// обновляем баланс
	sql, args, err = sq.Update("accounts").
		Set("balance", currentb).
		Where(sq.Eq{"userid": userto}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	// добавить транзакцию начисленияs
	sql, args, err = sq.Insert("tnx").
		Columns("id", "pointaccount", "points", "commitdate", "typetnx", "transferid").
		Values(uuid.New(), account, points, time.Now(), model.ACCRUEL, transferId).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	tx.Commit(ctx)
	return nil

}

// Получить баланс
func (p *PointsDB) GetBalance(ctx context.Context, user string) (points float64, err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}

	row := conn.QueryRow(ctx, "SELECT balance FROM accounts WHERE userid = $1", user)
	err = row.Scan(&points)
	p.logger.Info("points",
		zap.Any("points", points),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("user %w", model.ErrNotFound)
		}
		return 0, err
	}
	return points, nil
}

// Получить транзакции
func (p *PointsDB) GetTnx(ctx context.Context, user string, from time.Time, to time.Time) (tnxs []model.PointTransaction, err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	var account uuid.UUID
	var pguuid pgtype.UUID
	row := conn.QueryRow(ctx, "SELECT uuid from ACCOUNTS where userid = $1 FOR UPDATE", user)
	if err != nil {
		return nil, err
	}
	err = row.Scan(&pguuid)
	if err != nil {
		return nil, err
	}
	account, _ = uuid.FromBytes(pguuid.Bytes[:])
	sql, args, err := sq.Select("id", "pointaccount", "points", "commitdate", "typetnx", "orderid", "transferid", "redeemid").
		From("tnx").
		Where(sq.Eq{"pointaccount": account}).
		Where(sq.Eq{"commit": true}).
		Where(sq.GtOrEq{"commitdate": from}).
		Where(sq.LtOrEq{"commitdate": to}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("transactions %w", model.ErrNotFound)
		}
		return nil, err
	}
	var tnx model.PointTransaction
	var OrderID pgtype.Text
	var TransferID pgtype.Text
	var RedeemID pgtype.Text
	for rows.Next() {
		err = rows.Scan(&tnx.UUID, &tnx.PointAccount, &tnx.Points, &tnx.CommitDate, &tnx.TypeTnx, &OrderID, &TransferID, &RedeemID)
		if err != nil {
			return nil, err
		}
		tnx.OrderID = OrderID.String
		tnx.TransferID = TransferID.String
		tnx.RedeemID = RedeemID.String
		tnxs = append(tnxs, tnx)
	}
	return tnxs, nil
}

// Получить UUID аккаунта
func (p *PointsDB) GetUserUUID(ctx context.Context, user string) (account uuid.UUID, err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	row := conn.QueryRow(ctx, "SELECT uuid FROM accounts WHERE userid = $1", user)
	var pguuid pgtype.UUID

	err = row.Scan(&pguuid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			account, err = p.UserCreate(ctx, user)
			return account, err
		}
		return uuid.Nil, err
	}
	account, _ = uuid.FromBytes(pguuid.Bytes[:])
	return account, nil
}
