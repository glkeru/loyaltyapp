package engine

import (
	"context"
	"testing"

	models "github.com/glkeru/loyalty/engine/internal/models"
	uuid "github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestCompareValues(t *testing.T) {
	tests := []struct {
		value1   any
		value2   any
		expected int
	}{
		{"2025-01-02", "2025-03-03", -1},
		{"2025-01-02", "2025-01-02", 0},
		{"2025-01-02", "2025-01-01", 1},
		{344.3, 200, 1},
		{344.3, 344.3, 0},
		{200, 344.3, -1},
		{2, 1, 1},
		{true, true, 0},
		{true, false, -1},
		{"StrEqual", "StrEqual", 0},
		{"StrEqual", "StrNotEqual", -1},
	}

	for _, ts := range tests {
		result, err := compareValues(ts.value1, ts.value2)
		require.NoError(t, err, "value1=%v value2=%v", ts.value1, ts.value2)
		require.Equal(t, result, ts.expected, "value1=%v value2=%v", ts.value1, ts.value2)
	}
}

func TestCompareValuesErrors(t *testing.T) {
	tests := []struct {
		value1   any
		value2   any
		expected int
	}{
		{"2025-01-02", true, 1},
		{"2025-01-02", 244.43, 1},
		{false, 244.43, 1},
	}

	for _, ts := range tests {
		_, err := compareValues(ts.value1, ts.value2)
		require.Error(t, err, "value1=%v value2=%v", ts.value1, ts.value2)
	}
}

func TestCheckCondition(t *testing.T) {
	tests := []struct {
		value2   any
		operator string
		value1   any
		expected bool
	}{
		{"2025-01-02", ">", "2025-03-03", false},
		{"2025-01-02", "=", "2025-01-02", true},
		{"2025-01-02", "<=", "2025-01-02", true},
		{344.3, "<", 200, false},
		{344.3, "=", 200, false},
		{344.3, ">=", 200, true},
		{true, "=", true, true},
		{true, "=", false, false},
		{"StrEqual", "=", "StrEqual", true},
		{"StrEqual", "=", "StrNotEqual", false},
	}

	for _, ts := range tests {
		result, err := checkCondition(ts.value1, ts.operator, ts.value2)
		require.NoError(t, err, "value1=%v %s value2=%v", ts.value1, ts.operator, ts.value2)
		require.Equal(t, result, ts.expected, "value1=%v %s value2=%v", ts.value1, ts.operator, ts.value2)
	}
}

type TestCase struct {
	Expected int32
	Name     string
	Data     map[string]any
}

func TestFull(t *testing.T) {
	cont := gomock.NewController(t)
	defer cont.Finish()

	rules := []models.Rule{
		{
			ID:      uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			Active:  true,
			Maximum: false,
			Name:    "Общее правило: 10 баллов за каждую позицию с исключениями",
			Header: models.RewardCriteria{
				Include: []models.Criteria{
					{
						Operator: "AND",
						Conditions: []models.Condition{
							{Field: "total", Operator: ">=", Value: 1},
						},
					},
				},
			},
			Items: []models.RewardCriteria{
				{
					Points: int32(10),
					Include: []models.Criteria{
						{
							Operator: "AND",
							Conditions: []models.Condition{
								{Field: "price", Operator: ">=", Value: 1},
							},
						},
					},
					Exclude: []models.Criteria{
						{
							Operator: "OR",
							Conditions: []models.Condition{
								{Field: "price", Operator: "<", Value: 100},
								{Field: "productid", Operator: "=", Value: "BadProduct"},
							},
						},
					},
				},
			},
		},
		{
			ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Active:  true,
			Maximum: true,
			Name:    "Максимальное правило: 50% на конкретный продукт",
			Header: models.RewardCriteria{
				Include: []models.Criteria{
					{
						Operator: "AND",
						Conditions: []models.Condition{
							{Field: "total", Operator: ">=", Value: 1},
						},
					},
				},
			},
			Items: []models.RewardCriteria{
				{
					Percent: int32(50),
					Include: []models.Criteria{
						{
							Operator: "AND",
							Conditions: []models.Condition{
								{Field: "productid", Operator: "=", Value: "MaxProduct"},
							},
						},
					},
				},
			},
		},
		{
			ID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			Active:  true,
			Maximum: false,
			Name:    "Общее правило: 10% за заказ",
			Header: models.RewardCriteria{
				Percent: int32(10),
				Include: []models.Criteria{
					{
						Operator: "AND",
						Conditions: []models.Condition{
							{Field: "total", Operator: ">=", Value: 1},
						},
					},
				},
			},
		},
		{
			ID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			Active:  true,
			Maximum: true,
			Name:    "Максимальное правило на дату: x2 за заказ",
			Header: models.RewardCriteria{
				Percent: int32(200),
				Include: []models.Criteria{
					{
						Operator: "AND",
						Conditions: []models.Condition{
							{Field: "total", Operator: ">=", Value: 1},
							{Field: "orderdate", Operator: ">=", Value: "2025-01-01"},
							{Field: "orderdate", Operator: "<=", Value: "2025-01-08"},
						},
					},
				},
			},
		},
		{
			ID:      uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			Active:  true,
			Maximum: true,
			Name:    "Максимальное правило на флаг: x3 за заказ, если стоит флаг",
			Header: models.RewardCriteria{
				Percent: int32(300),
				Include: []models.Criteria{
					{
						Operator: "AND",
						Conditions: []models.Condition{
							{Field: "total", Operator: ">=", Value: 1},
							{Field: "jackpot", Operator: "=", Value: true},
						},
					},
				},
			},
		},
	}

	tengine := NewMockRuleStorage(cont)
	logger := zap.NewNop()

	tengine.EXPECT().
		GetActiveRules(gomock.Any()).
		Return(rules, nil).
		AnyTimes()

	serv, err := NewRuleEngineService(tengine, logger)
	if err != nil {
		t.Fatalf("NewRuleEngineService() error = %v", err)
	}

	// заказы для теста
	tests := []TestCase{
		{
			Expected: 15000,
			Name:     "Максимальное на флаг",
			Data: map[string]any{
				"total":     float64(5000),
				"jackpot":   true,
				"orderdate": "2025-01-02",
				"items": []any{
					map[string]any{
						"itemid":    float64(1),
						"productid": "MaxProduct",
						"price":     float64(5000),
					},
				},
			},
		},
		{
			Expected: 102000,
			Name:     "Максимальное на дату",
			Data: map[string]any{
				"total":     float64(51000),
				"orderdate": "2025-01-02",
				"items": []any{
					map[string]any{
						"itemid":    float64(1),
						"productid": "MaxProduc2",
						"price":     float64(5000),
					},
				},
			},
		},
		{
			Expected: 60,
			Name:     "Обычное",
			Data: map[string]any{
				"total":     float64(500),
				"orderdate": "2025-12-02",
				"items": []any{
					map[string]any{
						"itemid":    float64(1),
						"productid": "MaxProduc2",
						"price":     float64(500),
					},
				},
			},
		},
		{
			Expected: 250,
			Name:     "Максимальное на продукт",
			Data: map[string]any{
				"total":     float64(500),
				"orderdate": "2025-12-02",
				"items": []any{
					map[string]any{
						"itemid":    float64(1),
						"productid": "MaxProduct",
						"price":     float64(500),
					},
				},
			},
		},
		{
			Expected: 50,
			Name:     "Исключающее продукт",
			Data: map[string]any{
				"total":     float64(500),
				"orderdate": "2025-12-02",
				"items": []any{
					map[string]any{
						"itemid":    float64(1),
						"productid": "BadProduct",
						"price":     float64(500),
					},
				},
			},
		},
	}

	for _, ts := range tests {
		ts := ts
		t.Run(ts.Name, func(t *testing.T) {
			t.Parallel()
			result := serv.Calculate(context.Background(), ts.Data)
			require.Equal(t, result, ts.Expected, "data=%v expected=%v result=%v", ts.Data, ts.Expected, result)
		})
	}

}
