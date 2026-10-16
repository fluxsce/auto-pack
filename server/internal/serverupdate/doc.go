// Package serverupdate 把发行包合进本机安装目录并重启控制台。
// 保留 configs/*.yaml、data、logs、run；替换 bin、web、script 与启停脚本。
// 须用发行包内的 start/restart 脚本在安装目录启动；go run 开发态不能在此升级。
package serverupdate
