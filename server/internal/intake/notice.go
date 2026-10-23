package intake

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	subsystemDataHub = "DATAHUB"
	resultSuccess    = "SUCCESS"
	resultFailed     = "FAILED"

	msgSuccess = "自动打包完成，war 已推送到项目路径"
	msgBuild   = "自动打包失败：war 构建未完成，未推送"
	msgUpload  = "推送失败：原因未知"
)

// Notice 是一笔外部打包通知。仓库地址不从报文读取。
type Notice struct {
	TaskID             string
	TaskNo             string
	PackTaskID         string
	SubsystemType      string
	PackagingMode      string
	DeploymentStrategy string
	OSSProjectRoot     string
	CustomerCode       string
	CustomerShortName  string
	Version            string
	CustomerBranch     string
	BaseRef            string
	BaseRefType        string
	SYSVAL             string
	Recipients         []string
	ResourceFiles      []string
	FTPProjectRoot     string
	Legacy             bool
	rowID              string // 本行 rec_id，领取时生成，不来自报文
}

// Receipt 写回执队列的报文。
type Receipt struct {
	TaskID             string        `json:"taskId"`
	TaskNo             string        `json:"taskNo"`
	PackTaskID         string        `json:"packTaskId"`
	SubsystemType      string        `json:"subsystemType"`
	PackagingMode      string        `json:"packagingMode"`
	DeploymentStrategy string        `json:"deploymentStrategy"`
	OSSProjectRoot     string        `json:"ossProjectRoot"`
	UploadPath         string        `json:"uploadPath,omitempty"`
	OSSURL             string        `json:"ossUrl,omitempty"`
	ResultStatus       string        `json:"resultStatus"`
	ResultMessage      string        `json:"resultMessage"`
	FinishedAt         string        `json:"finishedAt"`
	Metadata           []ReceiptMeta `json:"metadata,omitempty"`
}

// ReceiptMeta 是回执里一步的执行记录。
type ReceiptMeta struct {
	Step       int    `json:"step"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Elapsed    string `json:"elapsed,omitempty"`
}

func parseNotice(raw []byte) (Notice, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Notice{}, err
	}
	var n Notice
	n.TaskID = jsonString(fields["taskId"])
	n.TaskNo = jsonString(fields["taskNo"])
	n.PackTaskID = jsonString(fields["packTaskId"])
	n.SubsystemType = jsonString(fields["subsystemType"])
	n.PackagingMode = jsonString(fields["packagingMode"])
	n.DeploymentStrategy = jsonString(fields["deploymentStrategy"])
	n.OSSProjectRoot = jsonString(fields["ossProjectRoot"])
	n.CustomerCode = jsonString(fields["customerCode"])
	n.CustomerShortName = jsonString(fields["customerShortName"])
	n.Version = jsonString(fields["version"])
	n.CustomerBranch = jsonString(fields["customerBranch"])
	n.BaseRef = jsonString(fields["baseRef"])
	n.BaseRefType = jsonString(fields["baseRefType"])
	n.SYSVAL = jsonString(fields["SYSVAL"])
	n.Recipients = noticeRecipients(fields)
	n.ResourceFiles = noticeList(fields, "resourceFiles")
	n.FTPProjectRoot = firstJSON(fields, "ftpProjectRoot", "ftpPath", "ftpRoot")
	n.Legacy = present(fields["sourceMetadata"])
	return n, nil
}

func firstJSON(fields map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if s := jsonString(fields[key]); s != "" {
			return s
		}
	}
	return ""
}

const defaultMailDomain = "flux.com.cn"

// completeMailAddr 给没有域名的收件人补上 @flux.com.cn。已写完整邮箱的保持原样。
func completeMailAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	at := strings.Index(addr, "@")
	if at < 0 {
		return addr + "@" + defaultMailDomain
	}
	local := strings.TrimSpace(addr[:at])
	host := strings.TrimSpace(addr[at+1:])
	if local == "" {
		return ""
	}
	if host == "" {
		return local + "@" + defaultMailDomain
	}
	return local + "@" + host
}

func noticeList(fields map[string]json.RawMessage, key string) []string {
	return jsonAddresses(fields[key])
}

func noticeRecipients(fields map[string]json.RawMessage) []string {
	keys := []string{"mailTo", "emails", "receivers", "recipients", "notifyTo", "receiver", "email"}
	seen := map[string]struct{}{}
	var out []string
	for _, key := range keys {
		for _, addr := range jsonAddresses(fields[key]) {
			addr = completeMailAddr(addr)
			if addr == "" {
				continue
			}
			low := strings.ToLower(addr)
			if _, ok := seen[low]; ok {
				continue
			}
			seen[low] = struct{}{}
			out = append(out, addr)
		}
	}
	return out
}

func jsonAddresses(raw json.RawMessage) []string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return splitAddrs(one)
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		var out []string
		for _, item := range many {
			out = append(out, splitAddrs(item)...)
		}
		return out
	}
	return nil
}

func splitAddrs(raw string) []string {
	raw = strings.NewReplacer(";", ",", "；", ",", "，", ",").Replace(raw)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func present(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null"
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// validateDataHub 按 DataHub 报文规则检查。通过时返回空字符串。
func validateDataHub(n Notice) string {
	if n.Legacy {
		return "旧报文含 sourceMetadata"
	}
	if n.TaskID == "" || n.TaskNo == "" || n.PackTaskID == "" {
		return "taskId、taskNo、packTaskId 必须非空"
	}
	if n.SubsystemType != subsystemDataHub {
		return "subsystemType 必须为 DATAHUB"
	}
	if n.PackagingMode != "war" || n.DeploymentStrategy != "war" {
		return "packagingMode 与 deploymentStrategy 必须为 war"
	}
	if n.OSSProjectRoot == "" && n.FTPProjectRoot == "" {
		return "ossProjectRoot 或 ftpProjectRoot 必须非空"
	}
	if n.CustomerCode == "" || n.Version == "" {
		return "customerCode、version 必须非空"
	}
	if n.CustomerBranch == "" || n.BaseRef == "" || n.BaseRefType == "" {
		return "customerBranch、baseRef、baseRefType 必须非空"
	}
	if !strings.EqualFold(n.BaseRefType, "tag") {
		return "baseRefType 本期必须为 tag"
	}
	return ""
}

func dataHubReceipt(n Notice, status, message string, at time.Time, meta []ReceiptMeta, paths, urls string) Receipt {
	return Receipt{
		TaskID:             n.TaskID,
		TaskNo:             n.TaskNo,
		PackTaskID:         n.PackTaskID,
		SubsystemType:      subsystemDataHub,
		PackagingMode:      "war",
		DeploymentStrategy: "war",
		OSSProjectRoot:     firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot),
		UploadPath:         receiptList(paths, true),
		OSSURL:             receiptList(urls, false),
		ResultStatus:       status,
		ResultMessage:      clipMessage(message),
		FinishedAt:         at.In(receiptTZ).Format("2006-01-02 15:04:05"),
		Metadata:           meta,
	}
}

// receiptList 把多次推送拼成逗号分隔。路径补成绝对路径；链接保持原样。
func receiptList(raw string, absPath bool) string {
	var parts []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\\", "/"))
		if line == "" {
			continue
		}
		if absPath && !strings.HasPrefix(line, "/") {
			line = "/" + line
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, ",")
}

func firstNonEmptyPath(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func clipMessage(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= 500 {
		return s
	}
	return string([]rune(s)[:500])
}

var receiptTZ = time.FixedZone("CST", 8*3600)
