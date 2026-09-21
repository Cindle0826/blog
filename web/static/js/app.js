/* 公開站的全部 JavaScript。
 *
 * 刻意保持極小：整站的互動只有「切換主題」「頁首陰影」「導覽列高亮」
 * 「目錄跟隨捲動」四件事。內容本身完全不需要 JS 就能讀，
 * 這是 SEO 與 Core Web Vitals 的基本盤。 */

(function () {
  'use strict';

  const root = document.documentElement;

  /* ── 主題切換 ──────────────────────────────────────────
     初始值已經由 base.gohtml 裡的 inline script 設好了（避免閃白），
     這裡只負責「按下去換一個」。 */
  const toggle = document.querySelector('[data-theme-toggle]');
  if (toggle) {
    toggle.addEventListener('click', function () {
      const next = root.dataset.theme === 'dark' ? 'light' : 'dark';
      root.dataset.theme = next;
      try {
        localStorage.setItem('theme', next);
      } catch (e) {
        /* 無痕模式會擋 localStorage，換不換得成不影響當下這次切換 */
      }
    });
  }

  /* 使用者沒手動選過的話，跟著系統設定走 */
  try {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    media.addEventListener('change', function (e) {
      if (!localStorage.getItem('theme')) {
        root.dataset.theme = e.matches ? 'dark' : 'light';
      }
    });
  } catch (e) { /* 舊瀏覽器沒有 addEventListener on MediaQueryList */ }

  /* ── 頁首捲動後才出現分隔線 ───────────────────────────── */
  const header = document.querySelector('.site-header');
  if (header) {
    const onScroll = function () {
      if (window.scrollY > 8) {
        header.setAttribute('data-scrolled', '');
      } else {
        header.removeAttribute('data-scrolled');
      }
    };
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
  }

  /* ── 導覽列高亮 ────────────────────────────────────────
     在前端做而不是在模板做，是為了讓 CDN 可以快取同一份 HTML
     給所有頁面共用的頁首。 */
  const path = window.location.pathname;
  document.querySelectorAll('.nav__link').forEach(function (link) {
    const href = link.getAttribute('href');
    if (href === '/' ? path === '/' : path.startsWith(href)) {
      link.setAttribute('aria-current', 'page');
    }
  });

  /* ── 目錄跟隨捲動 ─────────────────────────────────────── */
  const tocLinks = document.querySelectorAll('.toc a');
  if (tocLinks.length && 'IntersectionObserver' in window) {
    const byId = new Map();
    tocLinks.forEach(function (a) {
      const id = decodeURIComponent(a.getAttribute('href').slice(1));
      if (id) byId.set(id, a);
    });

    const observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) return;
          tocLinks.forEach(function (a) { a.classList.remove('is-active'); });
          const active = byId.get(entry.target.id);
          if (active) active.classList.add('is-active');
        });
      },
      /* 只把「畫面上緣往下 15%」那一條線當判定區，
         否則長標題會讓兩個項目同時亮起來 */
      { rootMargin: '-15% 0px -80% 0px', threshold: 0 }
    );

    byId.forEach(function (_, id) {
      const heading = document.getElementById(id);
      if (heading) observer.observe(heading);
    });
  }
})();
