package intake

import (
	"context"
	"strconv"
	"strings"

	"auto-pack/server/internal/host/mns"
)

// mqSource 读取当前应使用的 mq 连接。测试可替换；正式路径读全部已启用的环境。
type mqSource func(context.Context) (mns.Config, int, bool)

// mqTarget 是一条要轮询的消息队列环境。回执、邮件和 OSS 都用这一条，不借用别的环境。
type mqTarget struct {
	id      string
	name    string
	cfg     mns.Config
	retries int
}

func (t mqTarget) key() string {
	if id := strings.TrimSpace(t.id); id != "" {
		return id
	}
	return mqFingerprint(t.cfg)
}

func (t mqTarget) label() string {
	if name := strings.TrimSpace(t.name); name != "" {
		return name
	}
	if q := strings.TrimSpace(t.cfg.RequestQueue); q != "" {
		return q
	}
	return t.key()
}

// listReadyMQ 列出要消费的队列。测试钩子 mqSrc 仍只提供一条；正式路径读每条已启用且字段齐全的 mq。
// 名称排序靠前的那条不再代表全部。字段不齐的跳过，并在 skipped 里带回名称，避免把其它队列一起停掉。
func (s *Service) listReadyMQ(ctx context.Context) (ready []mqTarget, skipped []string, err error) {
	if s == nil {
		return nil, nil, nil
	}
	if s.mqSrc != nil {
		cfg, retries, ok := s.mqSrc(ctx)
		if !ok {
			if mqPartial(cfg) {
				return nil, []string{"mq"}, nil
			}
			return nil, nil, nil
		}
		if retries <= 0 {
			retries = 3
		}
		return []mqTarget{{
			id: s.mqEnvID, name: cfg.RequestQueue, cfg: cfg, retries: retries,
		}}, nil, nil
	}
	return s.readReadyMQ(ctx)
}

func (s *Service) readReadyMQ(ctx context.Context) ([]mqTarget, []string, error) {
	if s == nil || s.envs == nil {
		return nil, nil, nil
	}
	rows, cfgs, err := s.envs.ListKindEnabled(ctx, "mq")
	if err != nil {
		return nil, nil, err
	}
	ready := make([]mqTarget, 0, len(rows))
	var skipped []string
	for i, row := range rows {
		cfg, retries, ok := readMQMap(cfgs[i])
		if !ok {
			if mqPartial(cfg) {
				skipped = append(skipped, row.EnvName)
			}
			continue
		}
		ready = append(ready, mqTarget{id: row.RecID, name: row.EnvName, cfg: cfg, retries: retries})
	}
	return ready, skipped, nil
}

func readMQMap(m map[string]any) (mns.Config, int, bool) {
	cfg := mns.MapConfig(m)
	return cfg, receiptRetries(m), mqReady(cfg)
}

func mqReady(cfg mns.Config) bool {
	return strings.TrimSpace(cfg.Endpoint) != "" &&
		strings.TrimSpace(cfg.RequestQueue) != "" &&
		strings.TrimSpace(cfg.ResultQueue) != "" &&
		strings.TrimSpace(cfg.AccessKeyID) != "" &&
		strings.TrimSpace(cfg.AccessKeySecret) != ""
}

func mqPartial(cfg mns.Config) bool {
	return strings.TrimSpace(cfg.Endpoint) != "" ||
		strings.TrimSpace(cfg.RequestQueue) != "" ||
		strings.TrimSpace(cfg.ResultQueue) != "" ||
		strings.TrimSpace(cfg.AccessKeyID) != "" ||
		strings.TrimSpace(cfg.AccessKeySecret) != ""
}

func mqFingerprint(cfg mns.Config) string {
	return strings.Join([]string{
		strings.TrimSpace(cfg.Endpoint),
		strings.TrimSpace(cfg.RequestQueue),
		strings.TrimSpace(cfg.ResultQueue),
		strings.TrimSpace(cfg.AccessKeyID),
		strings.TrimSpace(cfg.AccessKeySecret),
		strconv.Itoa(cfg.BatchSize),
	}, "\n")
}

func receiptRetries(m map[string]any) int {
	if n, ok := configInt(m, "receipt_retries"); ok && n > 0 {
		return n
	}
	return 3
}

func configString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

func configInt(m map[string]any, key string) (int, bool) {
	raw := configString(m, key)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}
