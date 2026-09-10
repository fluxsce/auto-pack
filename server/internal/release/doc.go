// Package release 是工作空间自动发版 L3：编排 notes/pom/publish，落库与按空间锁走 record。
//
// 子包职责：
//   - notes：版本说明（Version.txt / UpdateNotes）与提交说明
//   - pom：POM 版本改写
//   - publish：Git 发布（checkout/commit/tag/push）与步骤日志
//   - suggest：Tag 建议、规范化与校验
//   - order：Maven 构建顺序
//   - record：AP_WORKSPACE_RELEASE 落库与按工作空间 Coordinator
package release
