package common

// GetString safely extracts a string from nvPairs map
func GetString(nv map[string]interface{}, key string) string {
	if v, ok := nv[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetBool safely extracts a bool from nvPairs map
func GetBool(nv map[string]interface{}, key string) bool {
	if v, ok := nv[key]; ok {
		switch b := v.(type) {
		case bool:
			return b
		case string:
			return b == "true" || b == "True" || b == "1"
		}
	}
	return false
}

// GetInt safely extracts an int from nvPairs map
func GetInt(nv map[string]interface{}, key string) int {
	if v, ok := nv[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return 0
}
