package store

import "testing"

func TestMakeSlug(t *testing.T) {
	const id = "aBc123XyZ9kLmN0pQrSt"

	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"一般英文", "Hello, World!", "hello-world"},
		{"中英混合只留英文", "Cloud Run 冷啟動優化", "cloud-run"},
		{"小數點變連字號", "把 Cloud Run 冷啟動從 1.8s 壓到 240ms", "cloud-run-1-8s-240ms"},
		{"符號連成一串", "C++ vs C#", "c-vs-c"},
		{"只有數字", "2026!!!", "2026"},
		{"純中文退回 ID", "純中文標題", "post-aBc123Xy"},
		{"空字串退回 ID", "", "post-aBc123Xy"},
		{"多餘空白", "  Go   入門  ", "go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeSlug(tt.title, id)
			if got != tt.want {
				t.Errorf("makeSlug(%q) = %q，預期 %q", tt.title, got, tt.want)
			}
		})
	}
}
