package queue

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type Queue interface {
	Enqueue(stream string, data interface{}) error
	Consume(stream string, group string, consumer string, handler func(id string, data map[string]interface{}) error) error
	Publish(channel string, data interface{}) error
	Subscribe(channel string, handler func(data []byte)) error
	Close() error
}

type RedisQueue struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisQueue(addr string) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisQueue{
		client: client,
		ctx:    ctx,
	}, nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}

func (q *RedisQueue) Enqueue(stream string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return q.client.XAdd(q.ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"data": string(jsonData),
		},
	}).Err()
}

func (q *RedisQueue) Consume(stream string, group string, consumer string, handler func(id string, data map[string]interface{}) error) error {
	// Create consumer group if not exists
	q.client.XGroupCreateMkStream(q.ctx, stream, group, "0")

	for {
		streams, err := q.client.XReadGroup(q.ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    0,
		}).Result()

		if err != nil {
			return err
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				dataStr, ok := message.Values["data"].(string)
				if !ok {
					continue
				}

				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					log.Printf("ERROR - Failed to unmarshal job data from queue: %v", err)
					continue
				}

				if err := handler(message.ID, data); err != nil {
					log.Printf("ERROR - Failed to process job %s: %v", data["id"], err)
					// TODO: Handle error (retry, dead letter, etc.)
					continue
				}

				// Acknowledge message
				q.client.XAck(q.ctx, stream.Stream, group, message.ID)
				log.Printf("Job %s acknowledged and removed from queue", data["id"])
			}
		}
	}
}

func (q *RedisQueue) Publish(channel string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return q.client.Publish(q.ctx, channel, string(jsonData)).Err()
}

func (q *RedisQueue) Subscribe(channel string, handler func(data []byte)) error {
	pubsub := q.client.Subscribe(q.ctx, channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		handler([]byte(msg.Payload))
	}

	return nil
}
