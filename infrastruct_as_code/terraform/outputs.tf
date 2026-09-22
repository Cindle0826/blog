output "cloud_run_url" {
  description = "Public URL of the blog service. Use it as site_base_url until the custom domain is live, and as the rewrite target in firebase.json."
  value       = google_cloud_run_v2_service.server.uri
}

output "cloud_run_service_name" {
  description = "Service name for the Firebase Hosting rewrite and for gcloud run deploy."
  value       = google_cloud_run_v2_service.server.name
}

output "artifact_registry_repo" {
  description = "Base image path. Tag images as <this>/server:<tag>."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

output "server_service_account" {
  description = "Runtime identity of the Cloud Run service."
  value       = google_service_account.server.email
}

output "uploads_bucket" {
  description = "GCS bucket for post images. Goes in BLOG_UPLOADS_BUCKET."
  value       = google_storage_bucket.uploads.name
}

output "uploads_path_prefix" {
  description = "Path prefix under which uploaded images are served. Post markdown stores paths relative to the site, never the bucket's own URL — the bucket is private and its objects are not reachable from the internet."
  value       = "/images/"
}

output "firestore_database" {
  description = "Firestore database name. \"(default)\" is what the Go client uses when no database is specified."
  value       = google_firestore_database.default.name
}

// ── Firebase Web App 設定 ────────────────────────────────────
//
// 這四個值會直接寫進前端程式碼並進 git。它們**不是機密**。
//
// api_key 這個名字取得很差：它不是密鑰，是一個專案識別碼，用來告訴
// Google「這個請求要找哪個專案」。任何人打開網站都能從原始碼看到它，
// 這是設計如此。實際的安全來自兩層：Firebase 驗證 ID token 的簽章，
// 以及後端比對 ADMIN_UIDS 白名單。
//
// 真正需要保護的是 service account 金鑰——而我們從來沒有下載過那種東西。

output "firebase_api_key" {
  description = "Firebase Web API key. Public by design; goes in the admin SPA's source."
  value       = data.google_firebase_web_app_config.admin.api_key
}

output "firebase_auth_domain" {
  description = "Domain that hosts the Google sign-in redirect flow."
  value       = data.google_firebase_web_app_config.admin.auth_domain
}

output "firebase_app_id" {
  description = "Firebase-assigned identifier for the admin web app."
  value       = google_firebase_web_app.admin.app_id
}

output "firebase_config_json" {
  description = "The whole config as JSON, ready to paste into the admin SPA."
  value = jsonencode({
    apiKey     = data.google_firebase_web_app_config.admin.api_key
    authDomain = data.google_firebase_web_app_config.admin.auth_domain
    projectId  = var.project_id
    appId      = google_firebase_web_app.admin.app_id
  })
}
