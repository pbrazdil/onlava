package main

import (
	"fmt"
	"strings"
	"sync"
)

type temporalActivityLogCollapser struct {
	mu        sync.Mutex
	threshold int
	counts    map[string]int
}

func newTemporalActivityLogCollapser(threshold int) *temporalActivityLogCollapser {
	if threshold <= 0 {
		threshold = 3
	}
	return &temporalActivityLogCollapser{
		threshold: threshold,
		counts:    map[string]int{},
	}
}

func (c *temporalActivityLogCollapser) Filter(_ int, _ string, data []byte) []byte {
	if c == nil || len(data) == 0 {
		return data
	}
	key, workflowType, errorText, ok := temporalActivityErrorCollapseKey(string(data))
	if !ok {
		return data
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[key]++
	count := c.counts[key]
	if count <= c.threshold {
		return data
	}
	if count == c.threshold+1 {
		return []byte(fmt.Sprintf("scenery: suppressed repeated Temporal activity error (workflow_type=%s repeats=1): %s\n", workflowType, errorText))
	}
	return nil
}

func temporalActivityErrorCollapseKey(line string) (key, workflowType, errorText string, ok bool) {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	if !strings.Contains(lower, "activity") || (!strings.Contains(lower, "fail") && !strings.Contains(lower, "error")) {
		return "", "", "", false
	}
	workflowType = firstTemporalLogField(trimmed, lower, "workflow_type", "workflowtype", "WorkflowType", "workflowType")
	if workflowType == "" {
		workflowType = "unknown"
	}
	errorText = firstTemporalLogField(trimmed, lower, "error", "err", "message", "msg")
	if errorText == "" {
		errorText = compactWhitespace(trimmed)
	}
	return workflowType + "\x00" + errorText, workflowType, errorText, true
}

func firstTemporalLogField(line, lower string, names ...string) string {
	for _, name := range names {
		value := temporalLogField(line, lower, name, "=")
		if value != "" {
			return value
		}
		value = temporalLogField(line, lower, name, ":")
		if value != "" {
			return value
		}
	}
	return ""
}

func temporalLogField(line, lower, name, sep string) string {
	needle := strings.ToLower(name + sep)
	idx := strings.Index(lower, needle)
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(line[idx+len(name)+len(sep):])
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		value = value[1:]
		end := strings.IndexByte(value, '"')
		if end < 0 {
			return value
		}
		return value[:end]
	}
	end := len(value)
	for i, r := range value {
		if r == ' ' || r == '\t' || r == ',' {
			end = i
			break
		}
	}
	return strings.Trim(value[:end], `"'`)
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
