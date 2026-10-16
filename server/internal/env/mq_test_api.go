package env

import (
	"context"
	"errors"
	"strings"
	"time"

	hostmns "auto-pack/server/internal/host/mns"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/mqlog"
)

const mqManualTimeout = 30 * time.Second

// MQTestIn 是 MNS 手工生产 / 消费测试入参（草稿或已存行，同 Probe）。
type MQTestIn struct {
	WorkspaceRecID string         `json:"workspace_rec_id"`
	RecID          string         `json:"rec_id"`
	EnvKind        string         `json:"env_kind"`
	Config         map[string]any `json:"config"`
	Queue          string         `json:"queue"`  // request | result
	Body           string         `json:"body"`   // 生产：JSON 文本
	Delete         bool           `json:"delete"` // 消费后是否删除
	BatchSize      int            `json:"batch_size"`
}

// MQProduceResult 是生产测试结果。
type MQProduceResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	Queue     string `json:"queue,omitempty"`
	QueueName string `json:"queue_name,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// MQConsumeMessage 是一条消费到的消息。
type MQConsumeMessage struct {
	MessageID     string `json:"message_id"`
	Body          string `json:"body"`
	ReceiptHandle string `json:"receipt_handle,omitempty"`
	Deleted       bool   `json:"deleted"`
}

// MQConsumeResult 是消费测试结果。
type MQConsumeResult struct {
	OK        bool               `json:"ok"`
	Message   string             `json:"message"`
	Queue     string             `json:"queue,omitempty"`
	QueueName string             `json:"queue_name,omitempty"`
	Empty     bool               `json:"empty,omitempty"`
	Messages  []MQConsumeMessage `json:"messages,omitempty"`
}

// MQProduce 往请求或结果队列发送一条消息。
func (s *Service) MQProduce(ctx context.Context, actor iam.Actor, in MQTestIn) (MQProduceResult, error) {
	if err := s.authorizeWrite(ctx, actor, in.WorkspaceRecID); err != nil {
		return MQProduceResult{}, err
	}
	kind, cfg, err := s.prepareProbeConfig(ctx, ProbeIn{
		WorkspaceRecID: in.WorkspaceRecID,
		RecID:          in.RecID,
		EnvKind:        firstNonEmpty(in.EnvKind, "mq"),
		Config:         in.Config,
	})
	if err != nil {
		return MQProduceResult{}, err
	}
	if kind != "mq" {
		return MQProduceResult{OK: false, Message: "environment must be mq"}, nil
	}
	mc := hostmns.MapConfig(cfg)
	queueName, label, err := resolveMQTestQueue(mc, in.Queue)
	if err != nil {
		return MQProduceResult{OK: false, Message: err.Error()}, nil
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return MQProduceResult{OK: false, Message: "body required"}, nil
	}
	client, err := hostmns.New(mc)
	if err != nil {
		return MQProduceResult{OK: false, Message: err.Error()}, nil
	}
	pctx, cancel := context.WithTimeout(ctx, mqManualTimeout)
	defer cancel()
	id, err := client.Send(pctx, queueName, []byte(body))
	if err != nil {
		return MQProduceResult{OK: false, Message: err.Error(), Queue: label, QueueName: queueName}, nil
	}
	s.noteMQ(ctx, mqlog.Entry{
		EnvRecID: in.RecID, Direction: mqlog.DirectionProduce, QueueRole: label, QueueName: queueName,
		MessageID: id, Body: body, Source: mqlog.SourceManual, ActorUserID: actor.UserID,
	})
	return MQProduceResult{
		OK:        true,
		Message:   "produced",
		Queue:     label,
		QueueName: queueName,
		MessageID: id,
	}, nil
}

// MQConsume 从请求或结果队列拉取消息；可选删除。
func (s *Service) MQConsume(ctx context.Context, actor iam.Actor, in MQTestIn) (MQConsumeResult, error) {
	if err := s.authorizeWrite(ctx, actor, in.WorkspaceRecID); err != nil {
		return MQConsumeResult{}, err
	}
	kind, cfg, err := s.prepareProbeConfig(ctx, ProbeIn{
		WorkspaceRecID: in.WorkspaceRecID,
		RecID:          in.RecID,
		EnvKind:        firstNonEmpty(in.EnvKind, "mq"),
		Config:         in.Config,
	})
	if err != nil {
		return MQConsumeResult{}, err
	}
	if kind != "mq" {
		return MQConsumeResult{OK: false, Message: "environment must be mq"}, nil
	}
	mc := hostmns.MapConfig(cfg)
	queueName, label, err := resolveMQTestQueue(mc, in.Queue)
	if err != nil {
		return MQConsumeResult{OK: false, Message: err.Error()}, nil
	}
	batch := in.BatchSize
	if batch <= 0 {
		batch = mc.BatchSize
	}
	if batch <= 0 {
		batch = 10
	}
	client, err := hostmns.New(mc)
	if err != nil {
		return MQConsumeResult{OK: false, Message: err.Error()}, nil
	}
	pctx, cancel := context.WithTimeout(ctx, mqManualTimeout)
	defer cancel()

	var msgs []hostmns.Message
	// 不删除时用 Peek，避免与本服务请求队列消费者抢同一条、也不把消息藏进不可见窗口。
	if in.Delete {
		msgs, err = client.Receive(pctx, queueName, batch, 0)
	} else {
		msgs, err = client.Peek(pctx, queueName, batch)
	}
	if errors.Is(err, hostmns.ErrMessageNotExist) {
		return MQConsumeResult{OK: true, Message: emptyQueueMessage(label), Queue: label, QueueName: queueName, Empty: true}, nil
	}
	if err != nil {
		return MQConsumeResult{OK: false, Message: err.Error(), Queue: label, QueueName: queueName}, nil
	}
	out := make([]MQConsumeMessage, 0, len(msgs))
	for _, m := range msgs {
		item := MQConsumeMessage{
			MessageID:     m.MessageID,
			Body:          string(m.Body),
			ReceiptHandle: m.ReceiptHandle,
		}
		if in.Delete {
			if delErr := client.Delete(pctx, queueName, m.ReceiptHandle); delErr != nil {
				item.Deleted = false
				out = append(out, item)
				s.noteManualMQ(ctx, in.RecID, actor.UserID, label, queueName, in.Delete, item)
				return MQConsumeResult{
					OK:        false,
					Message:   "received but delete failed: " + delErr.Error(),
					Queue:     label,
					QueueName: queueName,
					Messages:  append(out, remainingViews(msgs, len(out))...),
				}, nil
			}
			item.Deleted = true
		}
		out = append(out, item)
		s.noteManualMQ(ctx, in.RecID, actor.UserID, label, queueName, in.Delete, item)
	}
	msg := "peeked"
	if in.Delete {
		msg = "consumed"
	}
	return MQConsumeResult{
		OK:        true,
		Message:   msg,
		Queue:     label,
		QueueName: queueName,
		Messages:  out,
	}, nil
}

func (s *Service) noteManualMQ(ctx context.Context, envID, actorID, label, queueName string, receive bool, item MQConsumeMessage) {
	direction := mqlog.DirectionPeek
	if receive {
		direction = mqlog.DirectionConsume
	}
	s.noteMQ(ctx, mqlog.Entry{
		EnvRecID:    envID,
		Direction:   direction,
		QueueRole:   label,
		QueueName:   queueName,
		MessageID:   item.MessageID,
		Body:        item.Body,
		Source:      mqlog.SourceManual,
		Deleted:     item.Deleted,
		ActorUserID: actorID,
	})
}

func emptyQueueMessage(label string) string {
	if label == "request" {
		return "queue empty (request queue is polled by pack intake; the message may already be taken)"
	}
	return "queue empty"
}

func remainingViews(msgs []hostmns.Message, from int) []MQConsumeMessage {
	if from >= len(msgs) {
		return nil
	}
	out := make([]MQConsumeMessage, 0, len(msgs)-from)
	for _, m := range msgs[from:] {
		out = append(out, MQConsumeMessage{
			MessageID:     m.MessageID,
			Body:          string(m.Body),
			ReceiptHandle: m.ReceiptHandle,
		})
	}
	return out
}

func resolveMQTestQueue(mc hostmns.Config, queue string) (name, label string, err error) {
	switch strings.ToLower(strings.TrimSpace(queue)) {
	case "", "request", "request_queue":
		name = strings.TrimSpace(mc.RequestQueue)
		label = "request"
	case "result", "result_queue":
		name = strings.TrimSpace(mc.ResultQueue)
		label = "result"
	default:
		return "", "", errors.New("queue must be request or result")
	}
	if name == "" {
		return "", label, errors.New(label + " queue not configured")
	}
	return name, label, nil
}
