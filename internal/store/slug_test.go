package store

import (
	"strings"
	"testing"
)

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
		{"純中文退回 ID（轉小寫）", "純中文標題", "post-abc123xy"},
		{"空字串退回 ID（轉小寫）", "", "post-abc123xy"},
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

func TestValidSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"cloud-run-cold-start", true},
		{"go-1-27", true},
		{"a", true},
		{"post-abc123xy", true},
		{strings.Repeat("a", maxSlugLen), true},

		{"", false},
		{"Hello", false},        // 大寫
		{"hello world", false},  // 空白
		{"hello_world", false},  // 底線
		{"-hello", false},       // 連字號開頭
		{"hello-", false},       // 連字號結尾
		{"hello--world", false}, // 連續連字號
		{"a/b", false},          // 斜線會切斷網址
		{"../x", false},         // 路徑穿越
		{"中文", false},           // 非 ASCII
		{strings.Repeat("a", maxSlugLen+1), false}, // 超過長度
	}

	for _, tt := range tests {
		if got := ValidSlug(tt.slug); got != tt.want {
			t.Errorf("ValidSlug(%q) = %v，預期 %v", tt.slug, got, tt.want)
		}
	}
}

// makeSlug 產生的 slug 一定要通過 ValidSlug。兩邊的規則各寫在一個地方，
// 只改其中一邊時（例如 2026-10-15 發現 post- 後面接的 ID 有大寫），這裡會先失敗。
func TestMakeSlugAlwaysValid(t *testing.T) {
	ids := []string{"aBc123XyZ9kLmN0pQrSt", "ZZZZZZZZ000000000000"}
	titles := []string{
		"Hello, World!",
		"把 Cloud Run 冷啟動從 1.8s 壓到 240ms",
		"純中文標題",
		"",
		"---",
		"C++ / C# / F#",
		strings.Repeat("long title ", 20),      // 超過 maxSlugLen，要被截斷
		strings.Repeat("a", maxSlugLen) + "-b", // 截斷點剛好落在連字號上
	}

	for _, id := range ids {
		for _, title := range titles {
			if s := makeSlug(title, id); !ValidSlug(s) {
				t.Errorf("makeSlug(%q, %q) = %q，不符合 ValidSlug", title, id, s)
			}
		}
	}
}
