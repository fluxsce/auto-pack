package intake

import (
	"runtime"
	"strconv"
	"strings"
)

// packCommand 把本单打包参数加到工程原命令前。空命令保持空，交给现有构建步骤。
func packCommand(base, customerCode, version, sysval string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		return windowsSet("SYSVAL", sysval) + "&& " +
			windowsSet("customerCode", customerCode) + "&& " +
			windowsSet("version", version) + "&& " + base
	}
	return "SYSVAL=" + strconv.Quote(sysval) +
		" customerCode=" + strconv.Quote(customerCode) +
		" version=" + strconv.Quote(version) +
		" " + base
}

func windowsSet(key, val string) string {
	val = strings.NewReplacer("\r", "", "\n", "", `"`, "").Replace(val)
	return `set "` + key + `=` + val + `"`
}
