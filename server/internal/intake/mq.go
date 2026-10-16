package intake

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"auto-pack/server/internal/host/mns"
)

// MNSQueue 轮询请求队列，业务收下后删除；回执写入结果队列。
// 连接来自已启用的 mq 环境，由调用方用 mns.MapConfig 解析后传入。
type MNSQueue struct {
	client       *mns.Client
	requestQueue string
	resultQueue  string
	batchSize    int
	idleWait     time.Duration // 空队列后再等，默认 2s；长轮询已经等过，测试可改短
	errWait      time.Duration // 收消息出错等待，默认 3s
	waitSeconds  int           // 单次拉取长轮询秒数，0 表示 20
	stop         context.CancelFunc
	traffic      func(context.Context, Traffic)
}

// Traffic 是一次队列收发，供消息留痕。
type Traffic struct {
	Direction string
	QueueRole string
	QueueName string
	MessageID string
	Body      []byte
	Deleted   bool
}

// SetTraffic 在生产或消费成功后回调。插入失败由回调自己处理。
func (q *MNSQueue) SetTraffic(fn func(context.Context, Traffic)) {
	if q == nil {
		return
	}
	q.traffic = fn
}

func (q *MNSQueue) note(ctx context.Context, ev Traffic) {
	if q == nil || q.traffic == nil {
		return
	}
	q.traffic(ctx, ev)
}

// OpenMNS 校验配置并创建客户端。
func OpenMNS(cfg mns.Config) (*MNSQueue, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errNoEndpoint
	}
	reqQ := strings.TrimSpace(cfg.RequestQueue)
	resQ := strings.TrimSpace(cfg.ResultQueue)
	if reqQ == "" || resQ == "" {
		return nil, errNoQueue
	}
	client, err := mns.New(mns.Config{
		Endpoint:        cfg.Endpoint,
		RequestQueue:    reqQ,
		ResultQueue:     resQ,
		AccessKeyID:     cfg.AccessKeyID,
		AccessKeySecret: cfg.AccessKeySecret,
		BatchSize:       cfg.BatchSize,
	})
	if err != nil {
		return nil, err
	}
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 10
	}
	return &MNSQueue{client: client, requestQueue: reqQ, resultQueue: resQ, batchSize: batch}, nil
}

// Listen 后台轮询请求队列。Handle 成功（业务已收下）后再删消息。
func (q *MNSQueue) Listen(handle func(context.Context, []byte) error) error {
	if q == nil || q.client == nil {
		return errNoEndpoint
	}
	ctx, cancel := context.WithCancel(context.Background())
	q.stop = cancel
	go q.loop(ctx, handle)
	return nil
}

var errIdle = errors.New("intake: idle")

func (q *MNSQueue) loop(ctx context.Context, handle func(context.Context, []byte) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		err := q.poll(ctx, handle)
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		wait := q.waitErr()
		if errors.Is(err, errIdle) {
			wait = q.waitIdle()
		} else {
			log.Printf("intake mns receive: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// visibilityHold 是 MNS 允许的最长不可见时间。自动构建远长于队列默认隐藏窗口，
// 不延长的话同一条消息会在构建过程中重新可见，被再次消费。
const visibilityHold = 43200

// poll 拉一批请求。空队列返回 errIdle。收下后先延长不可见时间，Handle 成功后再删除。
func (q *MNSQueue) poll(ctx context.Context, handle func(context.Context, []byte) error) error {
	if q == nil || q.client == nil {
		return errNoEndpoint
	}
	msgs, err := q.client.Receive(ctx, q.requestQueue, q.batchSize, q.receiveWait())
	if errors.Is(err, mns.ErrMessageNotExist) {
		return errIdle
	}
	if err != nil {
		return err
	}
	held := make([]mns.Message, 0, len(msgs))
	for _, msg := range msgs {
		if next, err := q.client.ChangeVisibility(ctx, q.requestQueue, msg.ReceiptHandle, visibilityHold); err != nil {
			log.Printf("intake mns visibility %s: %v", msg.MessageID, err)
		} else {
			msg.ReceiptHandle = next
		}
		held = append(held, msg)
	}
	for _, msg := range held {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("intake mns recv id=%s bytes=%d", msg.MessageID, len(msg.Body))
		deleted := false
		if err := handle(ctx, msg.Body); err != nil {
			log.Printf("intake mns handle %s: %v", msg.MessageID, err)
			q.note(ctx, Traffic{
				Direction: "consume", QueueRole: "request", QueueName: q.requestQueue,
				MessageID: msg.MessageID, Body: msg.Body,
			})
			continue
		}
		if err := q.client.Delete(ctx, q.requestQueue, msg.ReceiptHandle); err != nil {
			log.Printf("intake mns delete %s: %v", msg.MessageID, err)
		} else {
			deleted = true
		}
		q.note(ctx, Traffic{
			Direction: "consume", QueueRole: "request", QueueName: q.requestQueue,
			MessageID: msg.MessageID, Body: msg.Body, Deleted: deleted,
		})
	}
	return nil
}

// Publish 把回执 JSON 写入结果队列。
func (q *MNSQueue) Publish(ctx context.Context, body []byte) error {
	if q == nil || q.client == nil {
		return errNoEndpoint
	}
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	id, err := q.client.Send(sendCtx, q.resultQueue, body)
	if err != nil {
		return err
	}
	q.note(ctx, Traffic{
		Direction: "produce", QueueRole: "result", QueueName: q.resultQueue,
		MessageID: id, Body: body,
	})
	return nil
}

// Close 停止轮询。
func (q *MNSQueue) Close() {
	if q == nil {
		return
	}
	if q.stop != nil {
		q.stop()
		q.stop = nil
	}
}

func (q *MNSQueue) receiveWait() int {
	if q != nil && q.waitSeconds > 0 {
		return q.waitSeconds
	}
	return 20
}

func (q *MNSQueue) waitIdle() time.Duration {
	if q != nil && q.idleWait > 0 {
		return q.idleWait
	}
	return 2 * time.Second
}

func (q *MNSQueue) waitErr() time.Duration {
	if q != nil && q.errWait > 0 {
		return q.errWait
	}
	return 3 * time.Second
}
