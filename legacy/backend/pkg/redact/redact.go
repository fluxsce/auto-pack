package redact

import (
	"net/url"
	"regexp"
	"strings"
)

// urlWithUserinfo matches scheme://user:pass@ or scheme://token@ in log/error text.
var urlWithUserinfo = regexp.MustCompile(`(?i)((?:https?|ssh|git)://)[^@\s/]+@`)

// Redact removes credentials embedded in URLs and common secret patterns from free-form text.
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = urlWithUserinfo.ReplaceAllString(s, `${1}****@`)
	return s
}

// URL returns a log-safe repository URL (masks userinfo when present).
func URL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Redact(raw)
	}
	if u.User == nil {
		return raw
	}
	host := u.Host
	if host == "" {
		host = u.Hostname()
	}
	out := u.Scheme + "://****@" + host + u.EscapedPath()
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}

// CommandArgs returns log-safe argv for shell/git commands.
func CommandArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = commandArg(a)
	}
	return out
}

func commandArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return arg
	}
	if strings.Contains(arg, "://") && strings.Contains(arg, "@") {
		return URL(arg)
	}
	return Redact(arg)
}
