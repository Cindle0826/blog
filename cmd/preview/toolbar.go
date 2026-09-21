package main

// devToolbar 是只在預覽伺服器出現的浮動工具列，
// 讓我們可以即時比較不同字型方案與主題，不用改程式重跑。
//
// 這段 HTML 完全不會出現在正式站——它是由 preview server 在回應裡注入的。
const devToolbar = `
<div id="devbar">
  <div class="devbar__row">
    <span class="devbar__label">字型</span>
    <button data-font="a" title="標題 JetBrains Mono ／ 內文 Inter + 思源黑體">A</button>
    <button data-font="b" title="全站 JetBrains Mono">B</button>
    <button data-font="c" title="標題 JetBrains Mono ／ 內文思源宋體">C</button>
    <button data-font="d" title="JetBrains Mono 只用在程式碼">D</button>
  </div>
  <div class="devbar__row">
    <span class="devbar__label">主題</span>
    <button data-set-theme="light">淺</button>
    <button data-set-theme="dark">深</button>
  </div>
  <p class="devbar__hint" data-devbar-hint></p>
</div>
<style>
  #devbar {
    position: fixed; right: 1rem; bottom: 1rem; z-index: 999;
    display: flex; flex-direction: column; gap: .4rem;
    padding: .7rem .8rem;
    background: var(--bg-elev); color: var(--fg);
    border: 1px solid var(--border-strong); border-radius: 10px;
    box-shadow: 0 8px 28px rgba(0,0,0,.18);
    font-family: var(--font-mono); font-size: .72rem;
    line-height: 1.4;
  }
  #devbar .devbar__row { display: flex; align-items: center; gap: .3rem; }
  #devbar .devbar__label { color: var(--fg-subtle); width: 2.2rem; }
  #devbar button {
    min-width: 1.9rem; padding: .18rem .45rem;
    border: 1px solid var(--border); border-radius: 5px;
    background: var(--bg); color: var(--fg-muted);
    font-family: inherit; font-size: inherit; cursor: pointer;
  }
  #devbar button:hover { border-color: var(--accent); color: var(--accent); }
  #devbar button[aria-pressed="true"] {
    background: var(--accent); border-color: var(--accent); color: #fff;
  }
  #devbar .devbar__hint {
    margin: .1rem 0 0; max-width: 13rem;
    color: var(--fg-subtle); font-size: .66rem;
  }
  @media print { #devbar { display: none; } }
</style>
<script>
(function () {
  var root = document.documentElement;
  var hint = document.querySelector('[data-devbar-hint]');
  var notes = {
    a: 'A ─ 標題等寬、內文黑體。工程感與可讀性的平衡點。',
    b: 'B ─ 全站等寬。最有個性，但長篇中文會偏吃力。',
    c: 'C ─ 內文思源宋體。最適合長文閱讀，偏文青。',
    d: 'D ─ 等寬只留給程式碼。最保守也最安全。'
  };

  function syncFont() {
    var cur = root.dataset.font || 'a';
    document.querySelectorAll('#devbar [data-font]').forEach(function (b) {
      b.setAttribute('aria-pressed', String(b.dataset.font === cur));
    });
    if (hint) hint.textContent = notes[cur] || '';
  }

  function syncTheme() {
    document.querySelectorAll('#devbar [data-set-theme]').forEach(function (b) {
      b.setAttribute('aria-pressed', String(b.dataset.setTheme === root.dataset.theme));
    });
  }

  try { root.dataset.font = localStorage.getItem('devfont') || 'a'; } catch (e) { root.dataset.font = 'a'; }

  document.querySelectorAll('#devbar [data-font]').forEach(function (b) {
    b.addEventListener('click', function () {
      root.dataset.font = b.dataset.font;
      try { localStorage.setItem('devfont', b.dataset.font); } catch (e) {}
      syncFont();
    });
  });

  document.querySelectorAll('#devbar [data-set-theme]').forEach(function (b) {
    b.addEventListener('click', function () {
      root.dataset.theme = b.dataset.setTheme;
      try { localStorage.setItem('theme', b.dataset.setTheme); } catch (e) {}
      syncTheme();
    });
  });

  syncFont();
  syncTheme();
})();
</script>
`
