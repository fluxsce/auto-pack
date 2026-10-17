// Package intake 接收其它平台的打包通知，校验后构建、上传并回执。
//
// 不改作业的同步、构建、构建后动作、结束邮件和 depends_on。
// 队列走阿里云 MNS HTTP。Endpoint、请求/结果队列与 AK 只维护在已启用的 mq 环境，接入层不另存一份，改完下一轮轮询即生效。
// customerBranch 与工程 repo_branch 关联唯一工作空间，然后启动该工作空间的自动构建。构建结束后从与工作空间编码同名的最终工程上传。该工程配置了本机源码目录时，从该目录取包（其它工程的构建结果汇总在这里）；否则从工作空间检出目录取包。该同名工程是最终工程：中间工程失败不影响消息队列结果，只有最终工程构建成功并完成上传才回执成功。
// resourceFiles 为逗号分隔的资源文件名。写出时只推这些文件；未写时推送该工程下全部 war。
// 收件人字段是 mailTo，多个用逗号分隔。没有写 @ 域名的账号在处理时补成 @flux.com.cn。有 ossProjectRoot 只推 OSS，只有 ftpProjectRoot 才推 FTP。连接优先用该 mq 环境关联的 OSS 或 FTP / SFTP / SSH。
// 该 mq 环境关联了 SMTP 与邮件模板时，打包和推送都成功才发信。报文有 ossProjectRoot 用「OSS 已更新」，只有 ftpProjectRoot 用「FTP 已更新」。发件人是 DATAHUB 更新通知 <fluxdatahub@flux.com.cn>，更新范围固定为 DATAHUB服务。推送失败时回执写具体原因。OSS 覆盖前把已有对象复制到 BAK/{时间戳}/，邮件备份栏和回执推送步骤都写该路径；没有备份时不写。不附构建日志。构建日志留在平台。
// 回执写入结果队列。resultMessage 只写三步结论，例如「打包成功，推送成功，邮件已发送」。路径、备份、耗时和收件人在 metadata 数组里。同样的三步明细追加到这次工作空间构建日志末尾。同一 taskId 再次到达时不重复打包：已结束的按原结果再写回执并注明原因，尚未结束的不另开构建、也不把进行中误报为失败。
// 请求队列：收下后把不可见时间延到 MNS 上限，避免构建未完成时同一条被再次领取；业务 Handle 成功后再 Delete。空队列 MessageNotExist 属正常。
package intake
