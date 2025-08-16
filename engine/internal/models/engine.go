package engine

import "github.com/google/uuid"

//import "go.mongodb.org/mongo-driver/bson/primitive"

type Rule struct {
	Active  bool             `bson:"active" json:"active" `
	Maximum bool             `bson:"maximum" json:"maximum"`
	ID      uuid.UUID        `bson:"id" json:"id"`
	Name    string           `bson:"name" json:"name"`
	Header  RewardCriteria   `bson:"header" json:"header"`
	Items   []RewardCriteria `bson:"items" json:"items"`
}

type Criteria struct {
	Operator   string      `bson:"operator" json:"operator"`
	Conditions []Condition `bson:"conditions" json:"conditions"`
}

type RewardCriteria struct {
	Points  int32      `bson:"points" json:"points"`
	Percent int32      `bson:"percent" json:"percent"`
	Include []Criteria `bson:"include" json:"include"`
	Exclude []Criteria `bson:"exclude" json:"exclude"`
}

type Condition struct {
	Field    string `bson:"field" json:"field"`
	Operator string `bson:"operator" json:"operator"`
	Value    any    `bson:"value" json:"value"`
}
