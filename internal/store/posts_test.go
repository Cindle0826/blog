package store

import (
	"strings"
	"testing"
)

func TestValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"aBc123XyZ9kLmN0pQrSt", true}, // NewDoc() 產生的格式：20 個英數字元

		{"", false},
		{"short", false},
		{"aBc123XyZ9kLmN0pQrStX", false}, // 21 個字
		{"__reserved__12345678", false},  // Firestore 保留給系統的格式，直接查會回 InvalidArgument
		{"has.dot.in.it.123456", false},
		{"has/slash/in/it/1234", false},
		{"中文中文中文中文中文中文中文中文中文中文", false},
		{strings.Repeat("a", 1500), false},
	}

	for _, tt := range tests {
		if got := ValidID(tt.id); got != tt.want {
			t.Errorf("ValidID(%q) = %v，預期 %v", tt.id, got, tt.want)
		}
	}
}
