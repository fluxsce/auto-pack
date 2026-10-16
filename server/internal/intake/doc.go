// Package intake 接收其它平台的打包通知，校验后构建、上传并回执。
//
// 不改作业的同步、构建、构建后动作、结束邮件和 depends_on。
// 队列走阿里云 MNS HTTP。Endpoint、请求/结果队列与 AK 只维护在已启用的 mq 环境，接入层不另存一份，改完下一轮轮询即生效。
// customerBranch 与工程 repo_branch 关联唯一工作空间，然后启动该工作空间的自动构建。构建结束后从与工作空间编码同名的工程上传。该同名工程是最终工程：中间工程失败不影响消息队列结果，只有最终工程构建成功并完成上传才回执成功。
// resourceFiles 为逗号分隔的资源文件名。写出时只推这些文件；未写时推送该工程下全部 war。
// 收件人字段是 mailTo，多个用逗号分隔。没有写 @ 域名的账号在处理时补成 @flux.com.cn。OSS 项目传 ossProjectRoot；FTP 项目传 ftpProjectRoot。连接优先用该 mq 环境关联的 OSS 或 FTP / SFTP / SSH。
// 该 mq 环境关联了 SMTP 与邮件模板（FTP 已更新、OSS 已更新、项目已更新、推送通知）时，处理结束发信。OSS 已更新与 FTP 已更新同一版式，项目名称取 customerShortName。OSS 覆盖前把已有对象复制到 BAK/{时间戳}/，邮件备份栏写该路径。不附构建日志。构建日志留在平台。
// 回执写入结果队列，正文按 1 打包、2 推送、3 邮件 三步写结果。同一 taskId 再次到达时不重复打包：已结束的按原结果再写回执并注明原因，尚未结束的不另开构建、也不把进行中误报为失败。
// 请求队列：收下后把不可见时间延到 MNS 上限，避免构建未完成时同一条被再次领取；业务 Handle 成功后再 Delete。空队列 MessageNotExist 属正常。
package intake
