package store

import "strings"

const maxSlugLen = 60

// makeSlug 由標題產生 URL 用的 slug；標題裡沒有任何英數字元時，
// 退回 "post-" + id 前 8 碼。
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
		return "post-" + id[:8]
	}
	return slug
}
