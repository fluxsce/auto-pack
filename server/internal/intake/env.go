package intake

import (
	"context"
	"log"
	"strconv"
	"strings"

	"auto-pack/server/internal/env"
)

// Env 是队列和打包工程，来自全局环境里已启用的 mq。
type Env struct {
	Enabled   bool
	ProjectID string
	Retries   int
	Rocket    RocketConfig
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
	projectID := configString(m, "project_id")
	nameServer := configString(m, "name_server")
	notice := configString(m, "notice_topic")
	receipt := configString(m, "receipt_topic")
	return Env{
		Enabled:   projectID != "" && nameServer != "" && notice != "" && receipt != "",
		ProjectID: projectID,
		Retries:   retries,
		Rocket: RocketConfig{
			NameServer:   nameServer,
			NoticeTopic:  notice,
			ReceiptTopic: receipt,
			Group:        configString(m, "group"),
			AccessKey:    configString(m, "access_key"),
			SecretKey:    configString(m, "secret_key"),
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
