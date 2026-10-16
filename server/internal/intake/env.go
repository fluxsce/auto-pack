package intake

import (
	"context"
	"strconv"
	"strings"

	"auto-pack/server/internal/host/mns"
)

// mqSource 读取当前应使用的 mq 连接。测试可替换；正式路径读已启用的环境。
type mqSource func(context.Context) (mns.Config, int, bool)

// readMQ 读取已启用的 mq 环境。未配置或字段不齐时 ok 为 false，不订阅。
// 回执重试次数来自该环境的 receipt_retries，缺省 3。连接字段与环境页生产测试同一套 MapConfig。
func (s *Service) readMQ(ctx context.Context) (mns.Config, int, bool) {
	if s == nil || s.envs == nil {
		return mns.Config{}, 3, false
	}
	row, cfg, err := s.envs.FindKind(ctx, "", "mq")
	if err != nil || len(cfg) == 0 {
		s.mqEnvID = ""
		return mns.Config{}, 3, false
	}
	s.mqEnvID = row.RecID
	return readMQMap(cfg)
}

func (s *Service) currentMQ(ctx context.Context) (mns.Config, int, bool) {
	if s == nil {
		return mns.Config{}, 3, false
	}
	if s.mqSrc != nil {
		return s.mqSrc(ctx)
	}
	return s.readMQ(ctx)
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
