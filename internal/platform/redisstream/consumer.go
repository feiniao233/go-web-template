package redisstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrReject = errors.New("reject Redis Stream message")

type HandlerErrorFunc func(context.Context, redis.XMessage, error)

type Consumer struct {
	client         *redis.Client
	stream         string
	group          string
	consumer       string
	block          time.Duration
	count          int64
	retryIdle      time.Duration
	maxDeliveries  int64
	dlqStream      string
	onHandlerError HandlerErrorFunc
}

type Option func(*Consumer)

func WithHandlerError(fn HandlerErrorFunc) Option {
	return func(c *Consumer) { c.onHandlerError = fn }
}

func WithRetry(retryIdle time.Duration, maxDeliveries int64, dlqStream string) Option {
	return func(c *Consumer) {
		c.retryIdle = retryIdle
		c.maxDeliveries = maxDeliveries
		if dlqStream != "" {
			c.dlqStream = dlqStream
		}
	}
}

func Reject(err error) error {
	if err == nil {
		err = errors.New("message rejected")
	}
	return fmt.Errorf("%w: %w", ErrReject, err)
}

func New(client *redis.Client, stream, group, consumer string, block time.Duration, count int64, options ...Option) (*Consumer, error) {
	if client == nil || stream == "" || group == "" || consumer == "" || block <= 0 || count <= 0 {
		return nil, fmt.Errorf("invalid Redis Stream consumer settings")
	}
	result := &Consumer{
		client: client, stream: stream, group: group, consumer: consumer,
		block: block, count: count, dlqStream: stream + ":dlq",
	}
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	if result.retryIdle < 0 || result.maxDeliveries < 0 {
		return nil, fmt.Errorf("invalid Redis Stream retry settings")
	}
	if result.retryIdle > 0 && result.maxDeliveries <= 0 {
		return nil, fmt.Errorf("Redis Stream max deliveries must be positive when retry is enabled")
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
	if c.retryIdle == 0 {
		if err := c.drainPending(ctx, handle); err != nil {
			return err
		}
	}

	for {
		if c.retryIdle > 0 {
			retried, err := c.retryPending(ctx, handle)
			if err != nil {
				return err
			}
			if retried {
				continue
			}
		}

		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.group, Consumer: c.consumer, Streams: []string{c.stream, ">"}, Count: c.count, Block: c.block,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read Redis Stream: %w", err)
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if err := c.process(ctx, message, 1, handle); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Consumer) drainPending(ctx context.Context, handle func(context.Context, redis.XMessage) error) error {
	cursor := "0"
	for {
		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.group, Consumer: c.consumer, Streams: []string{c.stream, cursor}, Count: c.count, Block: -1,
		}).Result()
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read Redis Stream pending messages: %w", err)
		}
		received := false
		for _, stream := range streams {
			for _, message := range stream.Messages {
				received = true
				cursor = message.ID
				if err := c.process(ctx, message, 1, handle); err != nil {
					return err
				}
			}
		}
		if !received {
			return nil
		}
	}
}

func (c *Consumer) retryPending(ctx context.Context, handle func(context.Context, redis.XMessage) error) (bool, error) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.stream, Group: c.group, Start: "-", End: "+", Count: c.count, Consumer: c.consumer, Idle: c.retryIdle,
	}).Result()
	if errors.Is(err, redis.Nil) || len(pending) == 0 {
		return false, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return false, nil
		}
		return false, fmt.Errorf("read Redis Stream pending messages: %w", err)
	}

	for _, item := range pending {
		if c.maxDeliveries > 0 && item.RetryCount >= c.maxDeliveries {
			message, err := c.messageByID(ctx, item.ID)
			if err != nil {
				return false, err
			}
			if err := c.deadLetter(ctx, message, item.RetryCount, errors.New("maximum deliveries exceeded")); err != nil {
				return false, err
			}
			continue
		}

		messages, err := c.claim(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, message := range messages {
			if err := c.process(ctx, message, item.RetryCount+1, handle); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func (c *Consumer) claim(ctx context.Context, id string) ([]redis.XMessage, error) {
	messages, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream: c.stream, Group: c.group, Consumer: c.consumer, MinIdle: c.retryIdle, Messages: []string{id},
	}).Result()
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, fmt.Errorf("claim Redis Stream message %s: %w", id, err)
	}
	return messages, nil
}

func (c *Consumer) messageByID(ctx context.Context, id string) (redis.XMessage, error) {
	messages, err := c.client.XRangeN(ctx, c.stream, id, id, 1).Result()
	if err != nil {
		return redis.XMessage{}, fmt.Errorf("read Redis Stream message %s: %w", id, err)
	}
	if len(messages) == 0 {
		return redis.XMessage{ID: id, Values: map[string]any{}}, nil
	}
	return messages[0], nil
}

func (c *Consumer) process(ctx context.Context, message redis.XMessage, deliveryCount int64, handle func(context.Context, redis.XMessage) error) error {
	err := handle(ctx, message)
	if err == nil {
		return c.ack(ctx, message.ID)
	}
	if c.onHandlerError != nil {
		c.onHandlerError(ctx, message, err)
	}
	if errors.Is(err, ErrReject) {
		return c.deadLetter(ctx, message, deliveryCount, err)
	}
	return nil
}

func (c *Consumer) deadLetter(ctx context.Context, message redis.XMessage, deliveryCount int64, cause error) error {
	payload, err := json.Marshal(message.Values)
	if err != nil {
		return fmt.Errorf("encode Redis Stream message %s for DLQ: %w", message.ID, err)
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: c.dlqStream,
		Values: map[string]any{
			"original_stream": c.stream,
			"original_id":     message.ID,
			"group":           c.group,
			"consumer":        c.consumer,
			"delivery_count":  deliveryCount,
			"error":           cause.Error(),
			"failed_at":       time.Now().UTC().Format(time.RFC3339Nano),
			"payload_json":    string(payload),
		},
	}).Err(); err != nil {
		return fmt.Errorf("write Redis Stream message %s to DLQ: %w", message.ID, err)
	}
	return c.ack(ctx, message.ID)
}

func (c *Consumer) ack(ctx context.Context, id string) error {
	if err := c.client.XAck(ctx, c.stream, c.group, id).Err(); err != nil {
		return fmt.Errorf("ack Redis Stream message %s: %w", id, err)
	}
	return nil
}
