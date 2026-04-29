package storage

import (
	"strings"

	"omniflow-go/internal/config"
)

// MatchRoutingRules 按顺序匹配路由规则，返回首个匹配的 provider 别名。
// 未匹配返回空字符串，调用方应回退到 default provider。
func MatchRoutingRules(rules []config.RoutingRule, fileSize int64, ext string, mimeType string) string {
	normalizedExt := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))

	for _, rule := range rules {
		if matchConditions(rule.Conditions, fileSize, normalizedExt, mimeType) {
			return rule.TargetProvider
		}
	}
	return ""
}

func matchConditions(cond config.RuleConditions, fileSize int64, ext string, mimeType string) bool {
	if cond.MinFileSizeBytes > 0 && fileSize < cond.MinFileSizeBytes {
		return false
	}
	if cond.MaxFileSizeBytes > 0 && fileSize > cond.MaxFileSizeBytes {
		return false
	}

	if len(cond.Extensions) > 0 {
		matched := false
		for _, e := range cond.Extensions {
			if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(e), "."), ext) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(cond.MIMEPrefixes) > 0 {
		matched := false
		lower := strings.ToLower(strings.TrimSpace(mimeType))
		for _, prefix := range cond.MIMEPrefixes {
			if strings.HasPrefix(lower, strings.ToLower(strings.TrimSpace(prefix))) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}
