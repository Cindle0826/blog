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

func TestWithSuffix(t *testing.T) {
	tests := []struct {
		name string
		base string
		n    int
		want string
	}{
		{"一般長度直接加", "hello-world", 2, "hello-world-2"},
		{"兩位數", "hello-world", 10, "hello-world-10"},
		{"60 字加 -2 要截成 58 字", strings.Repeat("a", 60), 2, strings.Repeat("a", 58) + "-2"},
		{"60 字加 -10 要截成 57 字", strings.Repeat("a", 60), 10, strings.Repeat("a", 57) + "-10"},
		{"剛好放得下不用截", strings.Repeat("a", 58), 2, strings.Repeat("a", 58) + "-2"},
		{"差一個字要截一個", strings.Repeat("a", 59), 2, strings.Repeat("a", 58) + "-2"},
		// 截斷點落在連字號上：不去掉的話會變成 "--2"，ValidSlug 不會過
		{"截在連字號上要去掉", strings.Repeat("a", 57) + "-bc", 2, strings.Repeat("a", 57) + "-2"},
		{"截完後面是多個連字號", strings.Repeat("a", 56) + "-b-cd", 2, strings.Repeat("a", 56) + "-b-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withSuffix(tt.base, tt.n)
			if got != tt.want {
				t.Errorf("withSuffix(%q, %d) = %q，預期 %q", tt.base, tt.n, got, tt.want)
			}
			if len(got) > maxSlugLen {
				t.Errorf("長度 %d 超過 %d", len(got), maxSlugLen)
			}
			if !ValidSlug(got) {
				t.Errorf("%q 不符合 ValidSlug", got)
			}
		})
	}
}

// resolveSlug 最多試到 -20，每一個候選都必須是合法 slug。
func TestWithSuffixAlwaysValid(t *testing.T) {
	bases := []string{
		"a",
		"hello-world",
		strings.Repeat("a", maxSlugLen),
		strings.Repeat("ab-", 20)[:maxSlugLen-1], // 結尾是 b，中間很多連字號
		makeSlug(strings.Repeat("long title ", 20), "aBc123XyZ9kLmN0pQrSt"),
		makeSlug("純中文標題", "aBc123XyZ9kLmN0pQrSt"),
	}
	for _, base := range bases {
		for n := 2; n <= 20; n++ {
			if s := withSuffix(base, n); !ValidSlug(s) {
				t.Errorf("withSuffix(%q, %d) = %q，不符合 ValidSlug", base, n, s)
			}
		}
	}
}

func TestMakeSlugMoreCases(t *testing.T) {
	const id = "aBc123XyZ9kLmN0pQrSt"
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"全形英數不算英數", "ＡＢＣ", "post-abc123xy"},
		{"大寫轉小寫", "GoLang TIPS", "golang-tips"},
		{"開頭結尾的符號不留連字號", "...hello...", "hello"},
		{"emoji 當成分隔", "go🚀rocks", "go-rocks"},
		{"底線變連字號", "snake_case_title", "snake-case-title"},
		{"重音字母不算英數", "café", "caf"},
		{"超過 60 字截斷", strings.Repeat("a", 70), strings.Repeat("a", 60)},
		{"截斷後結尾是連字號要去掉", strings.Repeat("a", 59) + " b", strings.Repeat("a", 59)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeSlug(tt.title, id); got != tt.want {
				t.Errorf("makeSlug(%q) = %q，預期 %q", tt.title, got, tt.want)
			}
		})
	}
}
