package view

// JSON-LD 結構化資料。
//
// 這是 Google 用來理解「這頁是什麼」的機器可讀描述。有了它，搜尋結果才可能
// 出現作者、發佈日期、麵包屑那些 rich result。純靠 <meta> 是做不到的。
//
// 之所以放在 Go 而不是硬拼在模板裡：JSON 在 gohtml 裡拼字串極易出錯，
// 少一個逗號整包結構化資料就失效，而且你不會收到任何錯誤訊息。

// JSONLD 回傳這一頁要輸出的結構化資料。
// Base 被各頁 ViewModel 嵌入，所以模板裡直接 {{ jsonLD .JSONLD }} 就能用。
func (b Base) JSONLD() any {
	graph := []any{b.websiteNode(), b.personNode()}

	if b.Meta.OGType == "article" {
		graph = append(graph, b.articleNode())
	}
	if len(b.Meta.Breadcrumb) > 0 {
		graph = append(graph, b.breadcrumbNode())
	}

	return map[string]any{
		"@context": "https://schema.org",
		"@graph":   graph,
	}
}

// websiteNode 描述整個網站。SearchAction 讓 Google 有機會在搜尋結果
// 直接附上站內搜尋框。
func (b Base) websiteNode() map[string]any {
	return map[string]any{
		"@type":       "WebSite",
		"@id":         b.Site.BaseURL + "/#website",
		"url":         b.Site.BaseURL + "/",
		"name":        b.Site.Name,
		"description": b.Site.Tagline,
		"inLanguage":  b.Site.Lang,
		"publisher":   map[string]any{"@id": b.Site.BaseURL + "/#person"},
		"potentialAction": map[string]any{
			"@type":       "SearchAction",
			"target":      map[string]any{"@type": "EntryPoint", "urlTemplate": b.Site.BaseURL + "/search?q={search_term_string}"},
			"query-input": "required name=search_term_string",
		},
	}
}

// personNode 描述你本人，同時當作網站的 publisher 與文章的 author。
func (b Base) personNode() map[string]any {
	n := map[string]any{
		"@type": "Person",
		"@id":   b.Site.BaseURL + "/#person",
		"name":  b.Site.Author.Name,
		"url":   b.Site.BaseURL + "/",
	}
	if b.Site.Author.Bio != "" {
		n["description"] = b.Site.Author.Bio
	}
	if b.Site.Author.Avatar != "" {
		n["image"] = absURL(b.Site.BaseURL, b.Site.Author.Avatar)
	}
	if links := b.socialURLs(); len(links) > 0 {
		n["sameAs"] = links
	}
	return n
}

// articleNode 描述這篇文章。只有 OGType == "article" 的頁面會輸出。
func (b Base) articleNode() map[string]any {
	n := map[string]any{
		"@type":            "BlogPosting",
		"@id":              b.Meta.Canonical + "#article",
		"mainEntityOfPage": b.Meta.Canonical,
		"url":              b.Meta.Canonical,
		"headline":         truncate(110, b.Meta.Title), // schema.org 建議 headline 不超過 110 字元
		"description":      b.Meta.Description,
		"inLanguage":       b.Site.Lang,
		"author":           map[string]any{"@id": b.Site.BaseURL + "/#person"},
		"publisher":        map[string]any{"@id": b.Site.BaseURL + "/#person"},
		"isPartOf":         map[string]any{"@id": b.Site.BaseURL + "/#website"},
	}
	if b.Meta.PublishedAt != nil {
		n["datePublished"] = isoDate(*b.Meta.PublishedAt)
	}
	// 沒改過的話，dateModified 就等於 datePublished——這個欄位不能留空，
	// Google 會拿它判斷內容新鮮度。
	if b.Meta.UpdatedAt != nil {
		n["dateModified"] = isoDate(*b.Meta.UpdatedAt)
	} else if b.Meta.PublishedAt != nil {
		n["dateModified"] = isoDate(*b.Meta.PublishedAt)
	}
	if b.Meta.OGImage != "" {
		n["image"] = b.Meta.OGImage
	}
	if len(b.Meta.Keywords) > 0 {
		n["keywords"] = b.Meta.Keywords
	}
	return n
}

// breadcrumbNode 描述麵包屑。首頁由這裡自動補在最前面，
// handler 端的 Meta.Breadcrumb 不需要包含首頁。
func (b Base) breadcrumbNode() map[string]any {
	items := []any{
		map[string]any{
			"@type":    "ListItem",
			"position": 1,
			"name":     b.Site.Name,
			"item":     b.Site.BaseURL + "/",
		},
	}
	for i, c := range b.Meta.Breadcrumb {
		item := map[string]any{
			"@type":    "ListItem",
			"position": i + 2,
			"name":     c.Label,
		}
		// 最後一節（當前頁）照規範不該有 item
		if c.Href != "" {
			item["item"] = absURL(b.Site.BaseURL, c.Href)
		}
		items = append(items, item)
	}
	return map[string]any{
		"@type":           "BreadcrumbList",
		"@id":             b.Meta.Canonical + "#breadcrumb",
		"itemListElement": items,
	}
}

// socialURLs 收集社群連結，給 Person.sameAs 用。
// RSS 與 email 不算「同一個人的其他身分」，要排除。
func (b Base) socialURLs() []string {
	var out []string
	for _, s := range b.Site.Social {
		if s.Icon == "rss" || s.Icon == "email" {
			continue
		}
		out = append(out, s.Href)
	}
	return out
}

// ResolvedOGImage 回傳這頁實際要用的 og:image 絕對網址。
// 頁面沒指定就退回站台預設圖，絕對不要讓 og:image 留空——
// 分享到 LINE / Slack / Twitter 會變成一片空白。
func (b Base) ResolvedOGImage() string {
	if b.Meta.OGImage != "" {
		return absURL(b.Site.BaseURL, b.Meta.OGImage)
	}
	return absURL(b.Site.BaseURL, "/static/img/og-default.png")
}

// ResolvedDescription 回傳這頁的描述，沒填就退回站台標語。
func (b Base) ResolvedDescription() string {
	if b.Meta.Description != "" {
		return b.Meta.Description
	}
	return b.Site.Tagline
}

// ResolvedOGType 回傳 og:type，沒填預設 website。
func (b Base) ResolvedOGType() string {
	if b.Meta.OGType != "" {
		return b.Meta.OGType
	}
	return "website"
}
