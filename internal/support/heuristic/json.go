package heuristic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func CompactJSON(content string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(content)); err != nil || buf.Len() == 0 || buf.Len() >= len(content) {
		return content, false
	}
	return buf.String(), true
}

func ParseJSON(content string) (any, error) {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func DescribeJSONValue(value any, maxKeys int) string {
	switch item := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if maxKeys > 0 && len(keys) > maxKeys {
			keys = keys[:maxKeys]
		}
		return "JSON Object Keys: " + strings.Join(keys, ", ")
	case []any:
		return fmt.Sprintf("JSON Array Length: %d", len(item))
	default:
		return fmt.Sprintf("JSON Scalar: %v", item)
	}
}
