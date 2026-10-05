interface ImportMetaEnv {
  /**
   * Firebase 設定裡叫 apiKey，但它不是密碼：是專案識別碼，build 時會寫進 JS，
   * 任何人打開後台都看得到。GCP Console 裡這把 key 叫「Browser key」，
   * 所以這裡用同一個名字，避免被當成需要保密的東西。
   */
  readonly VITE_FIREBASE_BROWSER_KEY?: string
  readonly VITE_FIREBASE_AUTH_DOMAIN?: string
  readonly VITE_FIREBASE_PROJECT_ID?: string
  readonly VITE_FIREBASE_APP_ID?: string
  /** npm run dev:mock 會設成 "1"：不連 Firebase、不打 API，用假資料 */
  readonly VITE_MOCK_API?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
