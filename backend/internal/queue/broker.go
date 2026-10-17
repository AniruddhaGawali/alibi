package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"backend/pkg/models"
	"github.com/redis/go-redis/v9"
)

type (
	Handler func(ctx context.Context, msg models.Message) error
	Broker  interface {
		Publish(ctx context.Context, topic string, payload []byte) error
		Consume(ctx context.Context, topic string, handler Handler) error
	}
)

type RedisListBroker struct {
	client  *redis.Client
	dlqName string
}

func NewRedisListBroker(client *redis.Client, dlqName string) *RedisListBroker {
	return &RedisListBroker{
		client:  client,
		dlqName: dlqName,
	}
}

func (b *RedisListBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	err := b.client.LPush(ctx, topic, payload).Err()
	return err
}

func (b *RedisListBroker) Consume(ctx context.Context, topic string, handler Handler) error {
	data, err := b.client.BRPop(ctx, time.Second*2, topic).Result()
	if err != nil {
		return err
	}

	err = handler(ctx, models.Message{
		ID:      data[0],
		Payload: []byte(data[1]),
	})
	if err != nil {
		var dlqErr *models.DlqError
		var wErr *models.WorkerError

		if errors.As(err, &dlqErr) {
			b.Publish(ctx, b.dlqName, []byte(data[1]))
			log.Printf("%v", err)
			return dlqErr
		} else if errors.As(err, &wErr) {
			payload, marshalErr := json.Marshal(wErr.Data)
			if marshalErr != nil {
				return marshalErr
			}
			b.Publish(ctx, topic, payload)
			return err
		} else {
			b.Publish(ctx, topic, []byte(data[1]))
			return err
		}
	}

	return nil
}
