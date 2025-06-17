package engine

import (
	"context"

	engine "github.com/glkeru/loyalty/engine/internal/models"
	"github.com/google/uuid"
)

type RuleEngine interface {
	Relevant(ctx context.Context, order map[string]interface{}) (points int, err error)
}

type RuleStorage interface {
	GetAllRules(ctx context.Context) ([]engine.Rule, error)
	GetActiveRules(ctx context.Context) ([]engine.Rule, error)
	SaveRule(ctx context.Context, rule engine.Rule) error
	GetRule(ctx context.Context, ruleId uuid.UUID) (rule engine.Rule)
}
