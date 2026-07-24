package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
