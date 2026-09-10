// Package mod 供各 HTTP 功能模块自注册路由。router 只 Apply，不写具体路径。
package mod

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"auto-pack/server/internal/audit"
	wsbuild "auto-pack/server/internal/build"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release"
	"auto-pack/server/internal/settings"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

const (
	// Root 为产品 API 前缀。
	Root = "/autopack/api"
	// PageRoot 为控制台静态页前缀。
	PageRoot = "/autopack/"
)

// Path 拼 API 路径，如 Path("jobs", "{id}", "log") → /autopack/api/jobs/{id}/log。
func Path(elem ...string) string {
	if len(elem) == 0 {
		return Root
	}
	return Root + "/" + strings.Join(elem, "/")
}

// TokenIssuer 签发登录 JWT。由 internal/token 实现，本包不引用 token 包以免把 HTTP 模块绑死。
type TokenIssuer interface {
	Issue(userRecID, loginName string) (string, time.Time, error)
}

// Deps 是模块 Init 的依赖。鉴权中间件由 router 注入。
type Deps struct {
	// Mux 为根路由表；模块在上面挂自己的路径。
	Mux *http.ServeMux
	// Authz 包装需登录的处理器。
	Authz func(http.Handler) http.Handler
	// Tokens 签发登录令牌；登录模块使用。
	Tokens TokenIssuer
	// Users 为账号服务。
	Users *user.Service
	// Workspaces 为空间与成员服务。
	Workspaces *workspace.Service
	// Projects 为工程配方服务。
	Projects *project.Service
	// Envs 为环境凭据服务。
	Envs *env.Service
	// Settings 为系统属性与全局变量。
	Settings *settings.Service
	// Jobs 为作业入队与查询。
	Jobs *job.Service
	// Audit 为操作审计。
	Audit *audit.Service
	// Release 为工作空间自动发版。
	Release *release.Service
	// Build 为工作空间自动构建。
	Build *wsbuild.Service
	// Frontend 为静态页根目录。
	Frontend string
	// StartedAt 为进程启动时间，探活用。
	StartedAt time.Time
}

// InitFunc 在 Apply 时由模块自己挂路由。
type InitFunc func(Deps)

var (
	mu      sync.Mutex
	modules = map[string]InitFunc{}
	order   []string
)

// Register 由各模块 route 的 init 调用。同名后注册覆盖先注册。
func Register(name string, fn InitFunc) {
	if name == "" || fn == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := modules[name]; !ok {
		order = append(order, name)
	}
	modules[name] = fn
}

// Apply 按字典序调用各模块 Init。
func Apply(d Deps) {
	if d.Mux == nil {
		return
	}
	mu.Lock()
	names := append([]string(nil), order...)
	fn := make(map[string]InitFunc, len(modules))
	for k, v := range modules {
		fn[k] = v
	}
	mu.Unlock()
	sort.Strings(names)
	for _, name := range names {
		f := fn[name]
		if f == nil {
			continue
		}
		f(d)
	}
}

// Names 返回已注册模块名，按字典序，供测试。
func Names() []string {
	mu.Lock()
	defer mu.Unlock()
	out := append([]string(nil), order...)
	sort.Strings(out)
	return out
}
