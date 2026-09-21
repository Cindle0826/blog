package main

import (
	"fmt"
	"strings"

	"cindle.dev/blog/internal/view"
)

// 這個檔案是假資料。真資料上線後它仍然有價值：
// 改版面時不必連到 Firestore，而且內容刻意塞滿各種邊界情況
// （超長標題、沒有封面、沒有摘要、中英混排、表格、程式碼），
// 專門用來把版面做壞的地方逼出來。

func fixtureSite() view.Site {
	return view.Site{
		Name:     "Cindle",
		Tagline:  "SRE / Backend Engineer — 寫 Go，也寫踩過的坑",
		BaseURL:  baseURL,
		Lang:     "zh-Hant",
		BuildTag: "dev",
		Author: view.Author{
			Name:   "Cindle",
			Bio:    "後端與 SRE 工程師，主要寫 Go 與 Kubernetes。把踩過的坑寫下來，免得再踩一次。",
			Avatar: "/static/img/avatar.svg",
			Email:  "hi@example.com",
		},
		Nav: []view.NavItem{
			{Label: "文章", Href: "/posts"},
			{Label: "知識庫", Href: "/notes"},
			{Label: "標籤", Href: "/tags"},
			{Label: "關於", Href: "/about"},
		},
		Social: []view.SocialLink{
			{Label: "GitHub", Href: "https://github.com/", Icon: "github"},
			{Label: "RSS", Href: "/feed.xml", Icon: "rss"},
			{Label: "Email", Href: "mailto:hi@example.com", Icon: "email"},
		},
	}
}

var (
	tagGo    = view.Tag{Slug: "go", Name: "Go", Count: 12}
	tagK8s   = view.Tag{Slug: "kubernetes", Name: "Kubernetes", Count: 8}
	tagGCP   = view.Tag{Slug: "gcp", Name: "GCP", Count: 5}
	tagSRE   = view.Tag{Slug: "sre", Name: "SRE", Count: 7}
	tagPerf  = view.Tag{Slug: "performance", Name: "Performance", Count: 3}
	tagTerra = view.Tag{Slug: "terraform", Name: "Terraform", Count: 4}
)

func fixturePosts() []view.PostSummary {
	return []view.PostSummary{
		{
			Slug:        "cloud-run-cold-start",
			Title:       "把 Cloud Run 冷啟動從 1.8s 壓到 240ms",
			Summary:     "scale-to-zero 很香，但第一個倒楣的使用者要等將近兩秒。這篇記錄我怎麼拆解冷啟動的組成，以及哪些手段真的有效、哪些只是心理作用。",
			Tags:        []view.Tag{tagGCP, tagGo, tagPerf},
			PublishedAt: daysAgo(3),
			ReadingMin:  11,
			Pinned:      true,
			Cover: &view.Image{
				URL: "/static/img/placeholder-1.svg", Alt: "冷啟動時間分解圖", Width: 800, Height: 500,
			},
		},
		{
			Slug:        "firestore-cost-trap",
			Title:       "Firestore 免費額度的三個陷阱",
			Summary:     "每天 50,000 次讀取聽起來很多，直到你發現一個沒加索引的查詢可以在三分鐘內把它燒光。",
			Tags:        []view.Tag{tagGCP},
			PublishedAt: daysAgo(12),
			ReadingMin:  7,
		},
		{
			Slug:        "go-http-timeouts",
			Title:       "net/http 的 timeout 到底有幾層，我畫了一張圖",
			Summary:     "ReadTimeout、WriteTimeout、IdleTimeout、ReadHeaderTimeout、還有 context 的 deadline — 這五個東西管的根本不是同一段時間。",
			Tags:        []view.Tag{tagGo, tagSRE},
			PublishedAt: daysAgo(24),
			ReadingMin:  14,
		},
		{
			// 故意的邊界情況：超長標題、沒摘要、沒封面
			Slug:        "k8s-probe-misconfiguration",
			Title:       "一個設錯的 readinessProbe 如何讓整個叢集在星期五下午四點連環重啟（以及我學到的三件事）",
			Tags:        []view.Tag{tagK8s, tagSRE},
			PublishedAt: daysAgo(40),
			ReadingMin:  19,
		},
		{
			Slug:        "terraform-state-recovery",
			Title:       "Terraform state 壞掉之後的自救流程",
			Summary:     "先深呼吸，然後不要跑 terraform apply。",
			Tags:        []view.Tag{tagTerra, tagSRE},
			PublishedAt: daysAgo(58),
			ReadingMin:  9,
		},
	}
}

func fixtureNotes() []view.PostSummary {
	return []view.PostSummary{
		{
			Slug: "note-kubectl", Title: "kubectl 常用指令速查",
			Summary: "自己會回頭翻的那些。持續補充。",
			Tags:    []view.Tag{tagK8s}, PublishedAt: daysAgo(6), ReadingMin: 4,
		},
		{
			Slug: "note-go-concurrency", Title: "Go 並行模式整理",
			Summary: "worker pool、fan-in/fan-out、errgroup、semaphore — 什麼情況該用哪個。",
			Tags:    []view.Tag{tagGo}, PublishedAt: daysAgo(17), ReadingMin: 12,
		},
		{
			Slug: "note-sre-oncall", Title: "On-call 值班 checklist",
			Tags: []view.Tag{tagSRE}, PublishedAt: daysAgo(30), ReadingMin: 5,
		},
	}
}

func fixtureTagList() []view.Tag {
	return []view.Tag{tagGo, tagK8s, tagSRE, tagGCP, tagTerra, tagPerf}
}

func fixtureBase(site view.Site, m view.Meta) view.Base {
	if m.Canonical == "" {
		m.Canonical = site.BaseURL + "/"
	}
	return view.Base{Site: site, Meta: m}
}

func fixtureHome(site view.Site) view.HomeVM {
	return view.HomeVM{
		Base: fixtureBase(site, view.Meta{
			Title:       site.Name,
			Description: site.Tagline,
			Canonical:   site.BaseURL + "/",
			OGType:      "website",
		}),
		Intro: `<p>嗨，我是 Cindle。白天做 SRE，維運跑在 GCP 與 Kubernetes 上的服務；晚上寫 Go，把白天踩到的坑補起來。</p>` +
			`<p>這裡放兩種東西：<strong>文章</strong>是有頭有尾、講一個完整問題的長篇；<strong>知識庫</strong>是會被我反覆回頭翻、持續更新的筆記。</p>`,
		RecentPosts: fixturePosts()[:3],
		RecentNotes: fixtureNotes(),
		TopTags:     fixtureTagList(),
	}
}

func fixtureList(site view.Site, heading, desc, path string) view.ListVM {
	posts := fixturePosts()
	if strings.HasPrefix(path, "/notes") {
		posts = fixtureNotes()
	}
	return view.ListVM{
		Base: fixtureBase(site, view.Meta{
			Title:       heading,
			Description: desc,
			Canonical:   site.BaseURL + path,
			OGType:      "website",
			Breadcrumb:  []view.Crumb{{Label: heading}},
		}),
		Heading:     heading,
		Description: desc,
		Posts:       posts,
		Pagination: view.Pagination{
			Page: 1, TotalPages: 3, NextURL: path + "?page=2",
		},
	}
}

func fixtureTags(site view.Site) view.TagIndexVM {
	return view.TagIndexVM{
		Base: fixtureBase(site, view.Meta{
			Title:       "標籤",
			Description: "依主題瀏覽所有文章與筆記。",
			Canonical:   site.BaseURL + "/tags",
			Breadcrumb:  []view.Crumb{{Label: "標籤"}},
		}),
		Tags: fixtureTagList(),
	}
}

func fixtureAbout(site view.Site) view.AboutVM {
	return view.AboutVM{
		Base: fixtureBase(site, view.Meta{
			Title:       "關於",
			Description: site.Author.Bio,
			Canonical:   site.BaseURL + "/about",
			Breadcrumb:  []view.Crumb{{Label: "關於"}},
		}),
		HTML: `<p>後端與 SRE 工程師，人在台灣。</p>` +
			`<p>平常的工作大概是：讓服務不要掛、掛了要知道、知道了要修得快。工具箱裡主要是 <code>Go</code>、<code>Kubernetes</code>、<code>Terraform</code>，再加上一堆用完即丟的 shell script。</p>` +
			`<h2 id="why">為什麼寫這個 blog</h2>` +
			`<p>因為同一個坑我踩過兩次。</p>`,
	}
}

func fixtureSearch(site view.Site) view.SearchVM {
	return view.SearchVM{
		Base: fixtureBase(site, view.Meta{
			Title:       "搜尋",
			Description: "搜尋站內所有文章與筆記。",
			Canonical:   site.BaseURL + "/search",
			NoIndex:     true, // 搜尋頁沒有索引價值
			Breadcrumb:  []view.Crumb{{Label: "搜尋"}},
		}),
	}
}

func fixtureError(site view.Site) view.ErrorVM {
	return view.ErrorVM{
		Base: fixtureBase(site, view.Meta{
			Title:     "找不到頁面",
			Canonical: site.BaseURL + "/404",
			NoIndex:   true,
		}),
		Code:    404,
		Message: "這個頁面不存在，或是已經被我搬走了。",
	}
}

func fixturePost(site view.Site, slug string) view.PostVM {
	all := fixturePosts()
	summary := all[0]
	for _, p := range all {
		if p.Slug == slug {
			summary = p
			break
		}
	}

	published := summary.PublishedAt
	updated := daysAgo(1)

	return view.PostVM{
		Base: fixtureBase(site, view.Meta{
			Title:       summary.Title,
			Description: summary.Summary,
			Canonical:   fmt.Sprintf("%s/posts/%s", site.BaseURL, summary.Slug),
			OGType:      "article",
			PublishedAt: &published,
			UpdatedAt:   &updated,
			Keywords:    []string{"Cloud Run", "Go", "冷啟動", "GCP"},
			Breadcrumb: []view.Crumb{
				{Label: "文章", Href: "/posts"},
				{Label: summary.Title},
			},
		}),
		Post: view.PostDetail{
			PostSummary: summary,
			UpdatedAt:   &updated,
			HasCode:     true,
			HTML:        fixtureArticleHTML,
			TOC: []view.TOCItem{
				{Level: 2, Text: "問題從哪來", Anchor: "問題從哪來"},
				{Level: 2, Text: "先量再改", Anchor: "先量再改"},
				{Level: 3, Text: "分解冷啟動", Anchor: "分解冷啟動"},
				{Level: 3, Text: "找出真正的大頭", Anchor: "找出真正的大頭"},
				{Level: 2, Text: "有效的三個手段", Anchor: "有效的三個手段"},
				{Level: 2, Text: "沒用的兩個手段", Anchor: "沒用的兩個手段"},
				{Level: 2, Text: "結果", Anchor: "結果"},
			},
			Related: all[1:4],
			Prev:    &all[1],
			Next:    &all[2],
		},
	}
}

// fixtureArticleHTML 模擬 goldmark 的輸出，刻意塞滿各種元素，
// 用來檢查 .prose 的樣式有沒有漏掉什麼。
const fixtureArticleHTML = `
<p>Cloud Run 的 <code>min-instances=0</code> 是成本控制的關鍵設定 —— 沒有流量就不收錢。
代價是：閒置一段時間之後，下一個進來的使用者要等容器從零啟動。</p>

<p>我量到的第一個數字是 <strong>1.8 秒</strong>。對一個部落格來說，這個數字很糟糕：
Google 的 Core Web Vitals 把 LCP 的「良好」門檻訂在 2.5 秒，光是冷啟動就吃掉七成。</p>

<h2 id="問題從哪來">問題從哪來<a class="anchor" href="#問題從哪來" aria-label="錨點">#</a></h2>

<p>先講結論：<em>大部分人以為是 Go 的問題，其實不是。</em>
Go 的執行檔啟動通常在 10ms 以內，真正慢的是它啟動之後做的那些事。</p>

<blockquote>
<p>在你量到數字之前，所有的效能討論都是猜測。</p>
</blockquote>

<h2 id="先量再改">先量再改<a class="anchor" href="#先量再改" aria-label="錨點">#</a></h2>

<h3 id="分解冷啟動">分解冷啟動<a class="anchor" href="#分解冷啟動" aria-label="錨點">#</a></h3>

<p>Cloud Run 的冷啟動可以拆成四段，每一段的優化手段完全不同：</p>

<ol>
<li><strong>拉 image</strong> — 跟 image 大小成正比</li>
<li><strong>啟動容器</strong> — 幾乎固定</li>
<li><strong>程式初始化</strong> — 你能控制的最大一塊</li>
<li><strong>第一個請求</strong> — 包含所有 lazy init</li>
</ol>

<p>我在 <code>main()</code> 開頭塞了一個最土法煉鋼的計時器：</p>

<pre><code class="language-go">var bootStart = time.Now()

func main() {
    // 每個初始化步驟都印出累積時間，
    // 不需要 tracing 就能看出哪一段最肥
    mark := func(step string) {
        log.Printf("boot: %-22s %v", step, time.Since(bootStart))
    }

    cfg := config.Load()
    mark("config")

    fs, err := firestore.NewClient(ctx, cfg.ProjectID)
    if err != nil {
        log.Fatal(err)
    }
    mark("firestore client")

    renderer, err := view.NewRenderer(cfg.Site)
    if err != nil {
        log.Fatal(err)
    }
    mark("templates")

    log.Printf("boot: total %v", time.Since(bootStart))
}
</code></pre>

<h3 id="找出真正的大頭">找出真正的大頭<a class="anchor" href="#找出真正的大頭" aria-label="錨點">#</a></h3>

<p>跑出來的結果讓我有點意外：</p>

<table>
<thead>
<tr><th>階段</th><th>耗時</th><th>佔比</th></tr>
</thead>
<tbody>
<tr><td>拉 image（340MB）</td><td>820ms</td><td>46%</td></tr>
<tr><td>Firestore client 初始化</td><td>610ms</td><td>34%</td></tr>
<tr><td>模板解析</td><td>180ms</td><td>10%</td></tr>
<tr><td>其他</td><td>190ms</td><td>10%</td></tr>
</tbody>
</table>

<p>Firestore client 那 610ms 幾乎全部花在取得 credentials 上 —— 它要去 metadata server
問一次 service account token。這件事沒辦法避免，但可以<strong>不要擋住啟動</strong>。</p>

<h2 id="有效的三個手段">有效的三個手段<a class="anchor" href="#有效的三個手段" aria-label="錨點">#</a></h2>

<p>第一，把 image 從 340MB 砍到 18MB。多階段建置 + <code>distroless/static</code>，
順便把模板 <code>go:embed</code> 進 binary：</p>

<pre><code class="language-dockerfile">FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# -s -w 去掉 debug symbol，大約再省 25%
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /app /app
ENTRYPOINT ["/app"]
</code></pre>

<p>第二，把 Firestore client 的建立搬到背景，用 <code>sync.Once</code> 保護，
讓 HTTP server 先開始聽。第三，模板改在 <code>init()</code> 解析一次就好。</p>

<h2 id="沒用的兩個手段">沒用的兩個手段<a class="anchor" href="#沒用的兩個手段" aria-label="錨點">#</a></h2>

<p>調大 CPU 配額：沒用，瓶頸在網路不在運算。
改用 <code>alpine</code> 而不是 <code>distroless</code>：差距在誤差範圍內，
但 <code>distroless</code> 的攻擊面小得多，沒有理由不用。</p>

<p>順帶一提，如果你按 <kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>R</kbd> 硬重新整理，
量到的數字會包含 CDN miss，不要被騙了。</p>

<hr>

<h2 id="結果">結果<a class="anchor" href="#結果" aria-label="錨點">#</a></h2>

<p>冷啟動從 1,800ms 降到 240ms。但真正讓 LCP 好看的其實是另一件事：
<strong>在前面掛一層 CDN</strong>。絕大多數請求根本不會碰到 Cloud Run，
冷啟動快不快，只影響那少數幾個倒楣的 cache miss。</p>

<p>這也是我想說的重點 —— <em>先想辦法不要執行，再想辦法執行得快。</em></p>
`
