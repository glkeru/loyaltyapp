package engine

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	engine "github.com/glkeru/loyalty/engine/internal/interfaces"
	models "github.com/glkeru/loyalty/engine/internal/models"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type RuleEngineService struct {
	Rules  []models.Rule
	logger *zap.Logger
}

func NewRuleEngineService(db engine.RuleStorage, logger *zap.Logger) (service *RuleEngineService, err error) {
	rules, err := db.GetActiveRules(context.Background())
	if err != nil {
		return nil, err
	}
	return &RuleEngineService{rules, logger}, nil
}

// log
func (s *RuleEngineService) Log(err error) {
	s.logger.Error("Rule Engine",
		zap.String("service", "Calculate"),
		zap.Error(err),
	)
}

// Расчет баллов по правилам
func (s *RuleEngineService) Calculate(ctx context.Context, order map[string]any) (points int32) {
	wg := &sync.WaitGroup{}
	count := len(s.Rules)
	wg.Add(count)

	var pointsAll int32              // сумма баллов по обычным правилам
	var pointsMax int32              // наибольшее кол-во баллов среди правил с типом Maximum
	maxCh := make(chan int32, count) // канал для правил с типом Maximum

	for _, rule := range s.Rules {
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
				p, err := Relevant(ctx, order, rule)
				if err != nil {
					s.Log(err)
					return
				}
				switch rule.Maximum {
				case true:
					maxCh <- p
				default:
					atomic.AddInt32(&pointsAll, p)
				}
			}
		}()
	}
	wg.Wait()
	close(maxCh)
	for v := range maxCh {
		if v > pointsMax {
			pointsMax = v
		}
	}

	// возвращаем максимальные баллы - сумма обычных правил vs максимальное из правил Maximum
	if pointsAll > pointsMax {
		return pointsAll
	} else {
		return pointsMax
	}
}

// Расчет одного правила
func Relevant(ctx context.Context, order map[string]any, rule models.Rule) (points int32, err error) {
	// Заголовок
	ok, err := checkRewardCriteria(ctx, rule.Header, order)
	if err != nil {
		return 0, fmt.Errorf("incorrect rule: %s, %v", rule.ID.String(), err)
	}
	if !ok {
		return 0, nil
	}
	// Баллы для заголовка
	if rule.Header.Percent != 0 {
		total := order["total"].(float64)
		points = int32(math.Ceil(total * float64(rule.Header.Percent) / 100))
	} else {
		points = rule.Header.Points
	}
	// Позиции
	items, ok := order["items"].([]any)
	if ok {
		g, errorctx := errgroup.WithContext(ctx)
		for _, item := range items {
			i := item.(map[string]any)
			for _, v := range rule.Items {
				select {
				case <-ctx.Done():
					return 0, nil
				default:
					i := i
					v := v
					g.Go(func() error {
						select {
						case <-errorctx.Done():
							return nil
						default:
							ok, err := checkRewardCriteria(ctx, v, i)
							if err != nil {
								return err
							}
							if ok {
								price := i["price"].(float64)
								if v.Percent != 0 {
									p := int32(math.Ceil(price * float64(v.Percent) / 100))
									atomic.AddInt32(&points, p)
								} else {
									atomic.AddInt32(&points, int32(v.Points))
								}
							}
							return nil
						}
					})
				}
			}
		}
		if err := g.Wait(); err != nil {
			return 0, fmt.Errorf("incorrect rule: %s, %w", rule.ID.String(), err)
		}
	}
	return points, nil
}

// Расчет наборов Exclude и Include
func checkRewardCriteria(ctx context.Context, reward models.RewardCriteria, data map[string]any) (bool, error) {
	if len(reward.Include) == 0 {
		return false, fmt.Errorf("rule is empty")
	}
	var exclude, include bool
	// канал отмены: если сработало исключающее условие, нет необходимости завершать проверку включающих условий
	cancelCh := make(chan struct{})
	g, errorctx := errgroup.WithContext(ctx)

	// Исключающие условия
	g.Go(func() error {
		for _, v := range reward.Exclude {
			select {
			case <-errorctx.Done():
				return nil
			default:
				ok, err := checkCriteria(v, data)
				if err != nil {
					close(cancelCh)
					return err
				}
				if ok {
					exclude = true // если хоть одно условие сработало, значит исключаем
					close(cancelCh)
					return nil
				}
			}
		}
		return nil
	})

	// Включающие условия
	g.Go(func() error {
		var find bool
		for _, v := range reward.Include {
			select {
			case <-errorctx.Done():
				return nil
			case <-cancelCh:
				return nil
			default:
				ok, err := checkCriteria(v, data)
				if err != nil {
					close(cancelCh)
					return err
				}
				if !ok {
					return nil // если хоть одно условие не сработало, значит не подходит
				} else {
					find = true
				}

			}
		}
		include = find
		return nil
	})

	if err := g.Wait(); err != nil {
		return false, err
	}
	if include && !exclude {
		return true, nil
	}
	return false, nil
}

// Проверка одного критерия
func checkCriteria(criteria models.Criteria, data map[string]any) (bool, error) {
	var relevant bool
	switch criteria.Operator {
	case "OR":
		for _, c := range criteria.Conditions {
			d, ok := data[c.Field]
			if ok {
				ok, err := checkCondition(c.Value, c.Operator, d)
				if ok {
					return true, nil
				}
				if err != nil {
					return false, fmt.Errorf("criteria is wrong: %v, %s, %w", c.Field, c.Operator, err)
				}
			}
		}
	case "AND":
		for _, c := range criteria.Conditions {
			d, ok := data[c.Field]
			if !ok {
				return false, nil
			}
			ok, err := checkCondition(c.Value, c.Operator, d)
			if !ok {
				return false, nil
			}
			if err != nil {
				return false, fmt.Errorf("criteria is wrong: %v, %s, %w", c.Field, c.Operator, err)
			}
			relevant = true
		}
	}
	return relevant, nil
}

// Проверка условий: string, date, bool, numeric
func checkCondition(cond any, operator string, field any) (bool, error) {
	result, err := compareValues(field, cond)
	if err != nil {
		return false, fmt.Errorf("condition is wrong: %w", err)
	}

	switch operator {
	case "=":
		return result == 0, nil
	case "!=":
		return result != 0, nil
	case ">":
		return result == 1, nil
	case "<":
		return result == -1, nil
	case ">=":
		return result == 1 || result == 0, nil
	case "<=":
		return result == -1 || result == 0, nil
	}

	return false, nil
}

// Если равны возвращаем 0, если value1 больше value2 возвращаем 1, если меньше -1
// Пробуем преобразовывать: в даты, в числа, в булеан, в строки
func compareValues(value1, value2 any) (int, error) {
	// даты
	value2DT, value2ok := value2.(string)
	value1DT, value1ok := value1.(string)
	if value2ok && value1ok {
		var tvalue2 time.Time
		var tvalue1 time.Time
		tvalue2, err := time.Parse("2006-01-02", value2DT) // если удалось распарсить дату из правила, значит в заказе тоже должна быть дата
		if err == nil {
			tvalue1, err = time.Parse("2006-01-02", value1DT)
			if err != nil {
				i, ok := value1.(int64) // пробуем UNIX time, исключаем зависимость от Mongo dateTime
				if ok {
					tvalue1 = time.UnixMilli(i)
				}
			}
			switch {
			case tvalue1.IsZero() || tvalue2.IsZero():
				return 0, fmt.Errorf("date parsing error")
			case !tvalue2.IsZero() && !tvalue1.IsZero():
				switch {
				case tvalue1.After(tvalue2):
					return 1, nil
				case tvalue1.Before(tvalue2):
					return -1, nil
				default:
					return 0, nil
				}
			}
		}
	}

	// числа
	numvalue1, value1ok := toFloat64(value1)
	numvalue2, value2ok := toFloat64(value2)
	if value1ok && value2ok {
		switch {
		case numvalue1 > numvalue2:
			return 1, nil
		case numvalue1 < numvalue2:
			return -1, nil
		default:
			return 0, nil
		}
	}

	// bool
	boolvalue1, value1ok := value1.(bool)
	boolvalue2, value2ok := value2.(bool)
	if value1ok && value2ok {
		switch {
		case boolvalue1 == boolvalue2:
			return 0, nil
		default:
			return -1, nil
		}
	}

	// string
	strvalue1, value1ok := value1.(string)
	strvalue2, value2ok := value2.(string)
	if value1ok && value2ok {
		switch {
		case strvalue1 > strvalue2:
			return 1, nil
		case strvalue1 < strvalue2:
			return -1, nil
		case strvalue1 == strvalue2:
			return 0, nil
		}
	}

	return 0, fmt.Errorf("compare is impossible")
}

// преобразование в float64
func toFloat64(a any) (float64, bool) {
	switch val := a.(type) {
	case int:
		return float64(val), true
	case float64:
		return val, true
	}
	return 0, false
}
