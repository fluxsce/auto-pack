package intake

import (
	"context"
	"log"
	"strconv"
	"strings"

	"auto-pack/server/internal/env"
)

// Env 是 MNS 队列连接，来自全局环境里已启用的 mq。
// 打包工程不在此配置：报文 customerBranch 按工程 repo_branch 关联到工作空间。
type Env struct {
	Enabled bool
	Retries int
	MNS     MNSConfig
}

// Load 读取全局 mq 环境。未配置或字段不齐时不订阅。
func Load(ctx context.Context, envs *env.Service) Env {
	if envs == nil {
		return Env{}
	}
	raw := envs.KindConfig(ctx, "", "mq")
	got := fromConfig(raw)
	if len(raw) > 0 && !got.Enabled {
		log.Printf("intake mq env incomplete")
	}
	return got
}

func fromConfig(m map[string]any) Env {
	retries := 3
	if n, ok := configInt(m, "receipt_retries"); ok && n > 0 {
		retries = n
	}
	batch := 10
	if n, ok := configInt(m, "batch_size"); ok && n > 0 {
		batch = n
	}
	endpoint := configString(m, "endpoint")
	requestQ := configString(m, "request_queue")
	resultQ := configString(m, "result_queue")
	ak := firstNonEmptyPath(configString(m, "access_key_id"), configString(m, "access_key"))
	sk := firstNonEmptyPath(configString(m, "access_key_secret"), configString(m, "secret_key"))
	return Env{
		Enabled: endpoint != "" && requestQ != "" && resultQ != "" && ak != "" && sk != "",
		Retries: retries,
		MNS: MNSConfig{
			Endpoint:        endpoint,
			RequestQueue:    requestQ,
			ResultQueue:     resultQ,
			AccessKeyID:     ak,
			AccessKeySecret: sk,
			BatchSize:       batch,
		},
	}
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
