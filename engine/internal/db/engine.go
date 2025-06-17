package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	engine "github.com/glkeru/loyalty/engine/internal/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RulesDB struct {
	mgo  *mongo.Client
	coll *mongo.Collection
}

func NewRulesDB() (*RulesDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mng := os.Getenv("ENGINE_MONGO")
	if mng == "" {
		return nil, fmt.Errorf("env ENGINE_MONGO is not set")
	}

	options := options.Client().ApplyURI("mongodb://" + mng)
	client, err := mongo.Connect(ctx, options)
	if err != nil {
		return nil, err
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	db := client.Database("engineDB")
	coll := db.Collection("rules")

	return &RulesDB{client, coll}, nil
}

func (r RulesDB) GetActiveRules(ctx context.Context) ([]engine.Rule, error) {
	var rules []engine.Rule
	filter := bson.M{"active": true}
	result, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	for result.Next(ctx) {
		var rule engine.Rule
		err := result.Decode(&rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r RulesDB) SaveRule(ctx context.Context, rule engine.Rule) error {
	// если ID пустой, значит новое правило
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
		_, err := r.coll.InsertOne(ctx, rule)
		if err != nil {
			return err
		}
		return nil
	}
	// Обновление
	filter := bson.M{"id": rule.ID}
	r.coll.UpdateOne(ctx, filter, rule)
	return nil
}

func (r RulesDB) GetRule(ctx context.Context, ruleId uuid.UUID) (rule engine.Rule) {
	filter := bson.M{"id": ruleId}
	r.coll.FindOne(ctx, filter).Decode(rule)
	return rule
}

func (r RulesDB) GetAllRules(ctx context.Context) ([]engine.Rule, error) {
	var rules []engine.Rule
	result, err := r.coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	for result.Next(ctx) {
		var rule engine.Rule
		err := result.Decode(&rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
