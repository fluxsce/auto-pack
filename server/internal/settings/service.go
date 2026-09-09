package settings

import (
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Service 是系统属性与全局变量的 L3 用例。
type Service struct {
	st    *store.Store
	guard *iam.Guard
}

// NewService 返回 settings Service。
func NewService(st *store.Store, guard *iam.Guard) *Service {
	return &Service{st: st, guard: guard}
}
