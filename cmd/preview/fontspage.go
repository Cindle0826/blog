package main

// fontsPage 是字型方案的並排比較頁，只存在於預覽伺服器。
//
// 為什麼不用工具列切換就好：切換是「前後比較」，要靠記憶，
// 差異小的時候根本看不出來。並排是「同時比較」，差異一眼就看到。
//
// 它直接吃 web/static/css/ 的真實樣式，所以看到的就是實際效果，
// 不是另外做一份示意圖。
const fontsPage = `<!DOCTYPE html>
<html lang="zh-Hant" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>字型方案比較</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="/static/css/fonts.css">
<link rel="stylesheet" href="/static/css/main.css">
<style>
  body { padding: 2rem 1.5rem 5rem; display: block; }
  .page { max-width: 78rem; margin-inline: auto; }
  .page > header { margin-bottom: 2rem; }
  .page > header h1 { font-size: 1.8rem; margin-bottom: .4rem; }
  .page > header p { color: var(--fg-muted); font-size: .9rem; max-width: 46rem; }

  .grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 1.25rem; }
  @media (max-width: 68rem) { .grid { grid-template-columns: 1fr; } }

  .sample {
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-elev);
    padding: 1.4rem 1.5rem 1.6rem;
  }
  .sample__tag {
    display: flex; align-items: baseline; gap: .6rem;
    margin-bottom: .2rem;
    font-family: var(--latin-mono), monospace;
  }
  .sample__tag b { font-size: 1.05rem; color: var(--accent); }
  .sample__tag span { font-size: .74rem; color: var(--fg-subtle); }
  .sample__stack {
    font-family: var(--latin-mono), monospace;
    font-size: .68rem; color: var(--fg-subtle);
    margin-bottom: 1.1rem; padding-bottom: .9rem;
    border-bottom: 1px dashed var(--border);
    line-height: 1.6; word-break: break-all;
  }

  /* 樣本本體：font-family 全部走 CSS 變數，
     所以每一格只要設 data-font 就會套到對應方案 */
  .demo h2 {
    font-family: var(--font-heading);
    letter-spacing: var(--heading-tracking);
    font-size: 1.32rem; line-height: 1.3; font-weight: 700;
    margin-bottom: .7rem;
  }
  .demo p {
    font-family: var(--font-body);
    letter-spacing: var(--body-tracking);
    line-height: var(--body-leading, 1.85);
    font-size: 1rem; margin-bottom: .8rem;
  }
  .demo .latin { color: var(--fg-muted); font-size: .92rem; }
  .demo code {
    font-family: var(--font-code, var(--latin-mono));
    font-size: .85em; padding: .12em .38em;
    background: var(--code-bg); border-radius: 4px;
  }
  .demo pre {
    font-family: var(--font-code, var(--latin-mono));
    background: var(--code-bg); border: 1px solid var(--border);
    border-radius: 8px; padding: .8rem 1rem; font-size: .8rem;
    line-height: 1.65; overflow-x: auto; margin-top: .9rem;
  }

  .themebar {
    position: fixed; right: 1rem; bottom: 1rem;
    display: flex; gap: .3rem; padding: .55rem .7rem;
    background: var(--bg-elev); border: 1px solid var(--border-strong);
    border-radius: 10px; box-shadow: 0 8px 28px rgba(0,0,0,.18);
    font-family: var(--latin-mono), monospace; font-size: .75rem;
  }
  .themebar button {
    padding: .2rem .6rem; border: 1px solid var(--border);
    border-radius: 5px; background: var(--bg); color: var(--fg-muted); cursor: pointer;
    font: inherit;
  }
  .themebar button[aria-pressed="true"] { background: var(--accent); border-color: var(--accent); color: #fff; }
</style>
</head>
<body>
<div class="page">
  <header>
    <h1>字型方案比較</h1>
    <p>四格內容完全相同，只有字型不同。注意看<strong>中文的筆畫</strong>（黑體是等粗的，宋體有粗細變化、末端有裝飾），
       以及<strong>英數字的間距</strong>（等寬字每個字母佔一樣寬，比例字不是）。</p>
  </header>

  <div class="grid">
    ` + sampleA + sampleB + sampleC + sampleD + `
  </div>
</div>

<div class="themebar">
  <button data-theme-set="light" aria-pressed="true">淺色</button>
  <button data-theme-set="dark">深色</button>
</div>

<script>
  document.querySelectorAll('[data-theme-set]').forEach(function (b) {
    b.addEventListener('click', function () {
      document.documentElement.dataset.theme = b.dataset.themeSet;
      document.querySelectorAll('[data-theme-set]').forEach(function (o) {
        o.setAttribute('aria-pressed', String(o === b));
      });
    });
  });
</script>
</body>
</html>`

// demoBody 是四格共用的樣本內容。刻意包含：
//   - 中英混排的句子（看銜接是否自然）
//   - 數字與版本號（等寬字的數字對齊最明顯）
//   - 行內程式碼與程式碼區塊
const demoBody = `
  <div class="demo">
    <h2>把 Cloud Run 冷啟動從 1.8s 壓到 240ms</h2>
    <p>scale-to-zero 很香，但第一個進來的使用者要等將近兩秒。我把冷啟動拆成四段之後發現，
       真正慢的不是 Go 本身 —— 執行檔啟動只花了 10ms，肥的是 image 拉取的 820ms
       和 Firestore client 初始化的 610ms。</p>
    <p class="latin">The cold start breakdown showed 46% in image pull, 34% in client init,
       and only 10% in template parsing. Go 1.23 / distroless / 18MB.</p>
    <p>解法是把 image 從 340MB 砍到 18MB，並把 <code>firestore.NewClient</code> 搬到背景執行。</p>
    <pre>RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o /app ./cmd/server</pre>
  </div>`

const sampleA = `
    <section class="sample" data-font="a">
      <div class="sample__tag"><b>方案 A</b><span>目前的預設</span></div>
      <div class="sample__stack">標題 JetBrains Mono ／ 內文 Inter ／ 中文 思源黑體</div>` + demoBody + `
    </section>`

const sampleB = `
    <section class="sample" data-font="b">
      <div class="sample__tag"><b>方案 B</b><span>工程師味最重</span></div>
      <div class="sample__stack">標題與內文都是 JetBrains Mono ／ 中文 思源黑體</div>` + demoBody + `
    </section>`

const sampleC = `
    <section class="sample" data-font="c">
      <div class="sample__tag"><b>方案 C</b><span>長文閱讀感最好</span></div>
      <div class="sample__stack">標題 JetBrains Mono ／ 內文 Noto Serif ／ 中文 思源宋體</div>` + demoBody + `
    </section>`

const sampleD = `
    <section class="sample" data-font="d">
      <div class="sample__tag"><b>方案 D</b><span>最保守</span></div>
      <div class="sample__stack">標題與內文都是 Inter ／ 中文 思源黑體 ／ 等寬只留給程式碼</div>` + demoBody + `
    </section>`
