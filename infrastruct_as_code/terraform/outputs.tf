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
