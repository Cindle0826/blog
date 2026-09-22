// Runtime identity for the Cloud Run service.
//
// Cloud Run defaults to the Compute Engine default service account, which
// carries roles/editor — effectively project admin. A public-facing web
// server should not have that: if the process is ever compromised, the blast
// radius is the whole project. This account gets Firestore and the uploads
// bucket, nothing else.
resource "google_service_account" "server" {
  project      = var.project_id
  account_id   = "blog-server"
  display_name = "Cloud Run runtime identity for cmd/server"

  depends_on = [google_project_service.apis]
}

# Read and write documents. Note this is roles/datastore.user, not
# datastore.owner — it cannot create or delete the database itself, or touch
# indexes. Those belong to Terraform.
resource "google_project_iam_member" "server_firestore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.server.email}"
}

# Scoped to the one bucket rather than granted project-wide, so a future
# bucket is not implicitly writable by the web server.
resource "google_storage_bucket_iam_member" "server_uploads" {
  bucket = google_storage_bucket.uploads.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.server.email}"
}

# Verifying a Firebase ID token normally needs no IAM at all — the Admin SDK
# fetches Google's public certs and checks the signature offline. This role
# is for the lookups beyond that: reading a user record to confirm the
# account still exists and has not been disabled.
resource "google_project_iam_member" "server_firebase_auth" {
  project = var.project_id
  role    = "roles/firebaseauth.viewer"
  member  = "serviceAccount:${google_service_account.server.email}"
}
