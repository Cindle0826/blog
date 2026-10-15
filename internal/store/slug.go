package store

import (
	"regexp"
	"strconv"
	"strings"
)

const maxSlugLen = 60

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// makeSlug 由標題產生 URL 用的 slug；標題裡沒有任何英數字元時，
// 退回 "post-" + id 前 8 碼（轉成小寫）。
func makeSlug(title, id string) string {
	var b strings.Builder
	lastHyphen := true // 一開始就當作剛寫過連字號，這樣開頭的符號不會產生連字號

	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
		} else if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	slug := strings.TrimRight(b.String(), "-")
	if len(slug) > maxSlugLen {
		slug = strings.TrimRight(slug[:maxSlugLen], "-")
	}
	if slug == "" {
		return "post-" + strings.ToLower(id[:8])
	}
	return slug
}

func ValidSlug(s string) bool {
	return len(s) <= maxSlugLen && slugPattern.MatchString(s)
}

func withSuffix(base string, n int) string {
	suffix := "-" + strconv.Itoa(n)
	keep := maxSlugLen - len(suffix)
	if len(base) > keep {
		base = strings.TrimRight(base[:keep], "-") // 截在連字號上的話要去掉
	}
	return base + suffix
}
