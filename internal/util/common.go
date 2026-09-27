package util

import (
	"os"
	"strings"
)

func GetEnvOr(key, def string) string {
	if v := os.Getenv(key); len(strings.TrimSpace(v)) != 0 {
		return v
	}
	return def
}
