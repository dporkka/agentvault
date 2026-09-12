package mcp

import "encoding/json"

func jsonMarshal(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

func jsonUnmarshal(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
