// Package host 是本机工具的根，与作业控制面平级。根下不放实现。
//
//	bash    本机命令
//	git     clone / fetch / tag（HTTPS token / SSH 私钥）
//	maven   改 POM、跑 mvn（dependency:tree JSON）
//	npm     跑 npm / pnpm / yarn
//	ftp     FTP 上传
//	ssh     SSH 拨号 / SFTP 上传
//	smtp    作业结束通知邮件
//	utils   文件 / 目录 / 压缩（路径 jail、列目录、读文本、zip）
//
// 各族无 HTTP、无权限表。新的本机能力只许在本目录开子包，不要摊回 internal/ 根上。
// 文件与目录操作、文件压缩、目录压缩放 utils，不要写进 workspace / job。
package host
