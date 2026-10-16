package intake

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"auto-pack/server/internal/host/mns"
)

// MNSConfig 是打包请求队列与结果队列（阿里云 MNS）。
type MNSConfig struct {
	Endpoint        string
	RequestQueue    string
	ResultQueue     string
	AccessKeyID     string
	AccessKeySecret string
	BatchSize       int
}

// MNSQueue 轮询请求队列，业务收下后删除；回执写入结果队列。
type MNSQueue struct {
	client       *mns.Client
	requestQueue string
	resultQueue  string
	batchSize    int
	idleWait     time.Duration // 空队列等待，默认 2s；测试可改短
	errWait      time.Duration // 收消息出错等待，默认 3s
	stop         context.CancelFunc
}

// OpenMNS 校验配置并创建客户端。
func OpenMNS(cfg MNSConfig) (*MNSQueue, error) {
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

func (q *MNSQueue) loop(ctx context.Context, handle func(context.Context, []byte) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		msgs, err := q.client.Receive(ctx, q.requestQueue, q.batchSize, 0)
		if errors.Is(err, mns.ErrMessageNotExist) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(q.waitIdle()):
			}
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("intake mns receive: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(q.waitErr()):
			}
			continue
		}
		for _, msg := range msgs {
			if ctx.Err() != nil {
				return
			}
			if err := handle(ctx, msg.Body); err != nil {
				log.Printf("intake mns handle %s: %v", msg.MessageID, err)
				continue
			}
			if err := q.client.Delete(ctx, q.requestQueue, msg.ReceiptHandle); err != nil {
				log.Printf("intake mns delete %s: %v", msg.MessageID, err)
			}
		}
	}
}

// Publish 把回执 JSON 写入结果队列。
func (q *MNSQueue) Publish(ctx context.Context, body []byte) error {
	if q == nil || q.client == nil {
		return errNoEndpoint
	}
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := q.client.Send(sendCtx, q.resultQueue, body)
	return err
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
