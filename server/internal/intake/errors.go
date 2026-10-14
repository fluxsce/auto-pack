package intake

import "errors"

var (
	errUnbound      = errors.New("未配置打包工程")
	errDisabled     = errors.New("打包工程已停用")
	errNoReceiptDir = errors.New("未配置回执目录")
	errNoNameServer = errors.New("未配置队列 NameServer")
	errNoTopic      = errors.New("未配置通知或回执主题")
)
