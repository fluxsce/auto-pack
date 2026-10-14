package intake

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// RocketConfig 是打包通知队列和回执队列。消息键使用 taskId。
type RocketConfig struct {
	NameServer   string
	NoticeTopic  string
	ReceiptTopic string
	Group        string
	AccessKey    string
	SecretKey    string
}

// RocketQueue 订阅打包通知，并把回执按 taskId 写回回执主题。
type RocketQueue struct {
	noticeTopic  string
	receiptTopic string
	producer     rocketmq.Producer
	consumer     rocketmq.PushConsumer
}

// OpenRocket 连接 NameServer。未配置地址时返回错误，调用方继续走目录通知。
func OpenRocket(cfg RocketConfig) (*RocketQueue, error) {
	servers := splitServers(cfg.NameServer)
	if len(servers) == 0 {
		return nil, errNoNameServer
	}
	notice := strings.TrimSpace(cfg.NoticeTopic)
	receipt := strings.TrimSpace(cfg.ReceiptTopic)
	if notice == "" || receipt == "" {
		return nil, errNoTopic
	}
	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = "auto-pack-intake"
	}
	ns, err := primitive.NewNamesrvAddr(servers...)
	if err != nil {
		return nil, err
	}
	prodOpts := []producer.Option{
		producer.WithNameServer(ns),
		producer.WithGroupName(group + "-receipt"),
		producer.WithInstanceName("intake-receipt"),
		producer.WithRetry(0),
		producer.WithSendMsgTimeout(10 * time.Second),
		producer.WithVIPChannel(false),
	}
	consOpts := []consumer.Option{
		consumer.WithNameServer(ns),
		consumer.WithGroupName(group),
		consumer.WithInstance("intake-notice"),
		consumer.WithVIPChannel(false),
		consumer.WithConsumeGoroutineNums(1),
		consumer.WithConsumeTimeout(2 * time.Hour),
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		cred := primitive.Credentials{AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey}
		prodOpts = append(prodOpts, producer.WithCredentials(cred))
		consOpts = append(consOpts, consumer.WithCredentials(cred))
	}
	p, err := rocketmq.NewProducer(prodOpts...)
	if err != nil {
		return nil, err
	}
	if err := p.Start(); err != nil {
		return nil, err
	}
	c, err := rocketmq.NewPushConsumer(consOpts...)
	if err != nil {
		_ = p.Shutdown()
		return nil, err
	}
	return &RocketQueue{noticeTopic: notice, receiptTopic: receipt, producer: p, consumer: c}, nil
}

// Listen 开始消费通知。处理失败时稍后重投。已有 taskId 仍写回执，由 Handle 确认。
func (q *RocketQueue) Listen(handle func(context.Context, []byte) error) error {
	if q == nil || q.consumer == nil {
		return errNoNameServer
	}
	err := q.consumer.Subscribe(q.noticeTopic, consumer.MessageSelector{}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			if msg == nil {
				continue
			}
			if err := handle(ctx, msg.Body); err != nil {
				return consumer.ConsumeRetryLater, nil
			}
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		return err
	}
	return q.consumer.Start()
}

// Publish 把回执写入回执主题，消息键为 taskId。
func (q *RocketQueue) Publish(ctx context.Context, body []byte) error {
	if q == nil || q.producer == nil {
		return errNoNameServer
	}
	msg := receiptMessage(q.receiptTopic, body)
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := q.producer.SendSync(sendCtx, msg)
	return err
}

// Close 停止消费并关闭生产者。
func (q *RocketQueue) Close() {
	if q == nil {
		return
	}
	if q.consumer != nil {
		_ = q.consumer.Shutdown()
	}
	if q.producer != nil {
		_ = q.producer.Shutdown()
	}
}

func receiptMessage(topic string, body []byte) *primitive.Message {
	msg := primitive.NewMessage(topic, body)
	if key := receiptTaskID(body); key != "" {
		msg.WithKeys([]string{key})
	}
	return msg
}

func receiptTaskID(body []byte) string {
	var probe struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	return strings.TrimSpace(probe.TaskID)
}

func splitServers(raw string) []string {
	raw = strings.ReplaceAll(raw, ";", ",")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
