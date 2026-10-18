// Package chat stores interrogation exchanges (P4.3). The interrogation service
// publishes an Event to Topic after each answer; Worker consumes the queue and
// writes the exchange to MongoDB; Handler serves the history back to the player.
package chat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// Topic is the Redis list chat events are published to.
	Topic = "queue:chat:events"
	// DLQ holds events that could not be stored after every retry.
	DLQ = "queue:chat:dlq"
	// Collection is the MongoDB collection of stored exchanges.
	Collection = "chat_messages"
	// MaxHistory caps how many messages one history request returns.
	MaxHistory = 200
)

// Event is one answered question, as published to Topic. It is also the stored
// document: the event id is the document _id, so a redelivered event is a no-op.
type Event struct {
	EventID     string    `json:"event_id" bson:"_id"`
	UserID      string    `json:"user_id" bson:"user_id"`
	CaseID      string    `json:"case_id" bson:"case_id"` // the case (stories.id), as in /case/:id
	PlayID      string    `json:"play_id" bson:"play_id"` // this play-through (active_case_locks.case_id)
	SuspectID   string    `json:"suspect_id" bson:"suspect_id"`
	SuspectName string    `json:"suspect_name" bson:"suspect_name"`
	UserQuery   string    `json:"user_query" bson:"user_query"`
	AIResponse  string    `json:"ai_response" bson:"ai_response"`
	Model       string    `json:"model" bson:"model"`
	Timestamp   time.Time `json:"timestamp" bson:"timestamp"`
}

// Message is one stored exchange as the client sees it.
type Message struct {
	ID          string    `json:"id"`
	CaseID      string    `json:"case_id"`
	SuspectID   string    `json:"suspect_id"`
	SuspectName string    `json:"suspect_name"`
	UserQuery   string    `json:"user_query"`
	AIResponse  string    `json:"ai_response"`
	Timestamp   time.Time `json:"timestamp"`
}

// MongoStore keeps exchanges in MongoDB.
type MongoStore struct{ coll *mongo.Collection }

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{coll: db.Collection(Collection)}
}

// EnsureIndexes creates the index the history queries use. It is idempotent;
// call it at startup.
func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1}, {Key: "case_id", Value: 1},
			{Key: "suspect_id", Value: 1}, {Key: "timestamp", Value: 1},
		},
		Options: options.Index().SetName("user_case_suspect_time"),
	})
	if err != nil {
		return fmt.Errorf("chat: create index: %w", err)
	}
	return nil
}

// Insert stores an exchange. Storing the same event twice is not an error.
func (s *MongoStore) Insert(ctx context.Context, ev Event) error {
	_, err := s.coll.InsertOne(ctx, ev)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

// SuspectOf reports which suspect one of the user's stored exchanges was with.
// found is false when the entry does not exist or belongs to another user or case.
func (s *MongoStore) SuspectOf(ctx context.Context, userID, caseID, messageID string) (suspectID string, found bool, err error) {
	var doc Event
	err = s.coll.FindOne(ctx, bson.D{{Key: "_id", Value: messageID}, {Key: "user_id", Value: userID}, {Key: "case_id", Value: caseID}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return doc.SuspectID, true, nil
}

// History returns the player's exchanges with one suspect of a case, oldest
// first: the latest limit of them when there are more. A non-empty playID keeps
// only that play-through's exchanges (a replayed case starts a blank conversation);
// empty returns every play-through.
func (s *MongoStore) History(ctx context.Context, userID, caseID, suspectID, playID string, limit int) ([]Message, error) {
	filter := bson.D{{Key: "user_id", Value: userID}, {Key: "case_id", Value: caseID}, {Key: "suspect_id", Value: suspectID}}
	if playID != "" {
		filter = append(filter, bson.E{Key: "play_id", Value: playID})
	}
	cur, err := s.coll.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	var docs []Event
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	slices.Reverse(docs)
	out := make([]Message, len(docs))
	for i, d := range docs {
		out[i] = Message{
			ID: d.EventID, CaseID: d.CaseID, SuspectID: d.SuspectID, SuspectName: d.SuspectName,
			UserQuery: d.UserQuery, AIResponse: d.AIResponse, Timestamp: d.Timestamp,
		}
	}
	return out, nil
}
