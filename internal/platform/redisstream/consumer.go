package redisstream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type HandlerErrorFunc func(context.Context, redis.XMessage, error)

type Consumer struct {
	client         *redis.Client
	stream         string
	group          string
	consumer       string
	block          time.Duration
	count          int64
	onHandlerError HandlerErrorFunc
}

type Option func(*Consumer)

func WithHandlerError(fn HandlerErrorFunc) Option {
	return func(c *Consumer) { c.onHandlerError = fn }
}

func New(client *redis.Client, stream, group, consumer string, block time.Duration, count int64, options ...Option) (*Consumer, error) {
	if client == nil || stream == "" || group == "" || consumer == "" || block <= 0 || count <= 0 {
		return nil, fmt.Errorf("invalid Redis Stream consumer settings")
	}
	result := &Consumer{client: client, stream: stream, group: group, consumer: consumer, block: block, count: count}
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	return result, nil
}

func (c *Consumer) Run(ctx context.Context, handle func(context.Context, redis.XMessage) error) error {
	if handle == nil {
		return fmt.Errorf("Redis Stream handler is required")
	}
	if err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create Redis Stream group: %w", err)
	}

	// First resume messages pending for this consumer. Once all of them have
	// been visited, switch to ">" for newly delivered messages. Handler
	// failures intentionally remain pending for a later retry and do not make
	// the whole worker/process fail.
	streamID := "0"
	for {
		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.group, Consumer: c.consumer, Streams: []string{c.stream, streamID}, Count: c.count, Block: c.block,
		}).Result()
		if errors.Is(err, redis.Nil) {
			streamID = ">"
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read Redis Stream: %w", err)
		}

		received := false
		for _, stream := range streams {
			for _, message := range stream.Messages {
				received = true
				if streamID != ">" {
					streamID = message.ID
				}
				if err := handle(ctx, message); err != nil {
					if c.onHandlerError != nil {
						c.onHandlerError(ctx, message, err)
					}
					continue
				}
				if err := c.client.XAck(ctx, c.stream, c.group, message.ID).Err(); err != nil {
					return fmt.Errorf("ack Redis Stream message %s: %w", message.ID, err)
				}
			}
		}
		if !received {
			streamID = ">"
		}
	}
}
