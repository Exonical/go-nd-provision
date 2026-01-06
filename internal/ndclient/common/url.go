package common

import "net/url"

// AddQuery appends query parameters to a path.
// Returns the path unchanged if vals is empty.
func AddQuery(path string, vals url.Values) string {
	if len(vals) == 0 {
		return path
	}
	return path + "?" + vals.Encode()
}
