package intake

import "errors"

var (
	errNoBranchWorkspace  = errors.New("未找到 customerBranch 对应的工作空间")
	errAmbiguousWorkspace = errors.New("customerBranch 对应多个工作空间")
	errAmbiguousProject   = errors.New("工作空间内无法唯一确定打包工程")
	errBuildBusy          = errors.New("该工作空间正在自动构建")
	errNoWorkspaceBuild   = errors.New("未配置工作空间自动构建")
	errNoPackProject      = errors.New("未找到与工作空间编码同名的打包工程")
	errDisabled           = errors.New("打包工程已停用")
	errNoReceiptDir       = errors.New("未配置回执目录")
	errNoEndpoint         = errors.New("未配置 MNS Endpoint")
	errNoQueue            = errors.New("未配置请求或结果队列")
)
