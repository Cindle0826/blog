resource "google_storage_bucket" "uploads" {
  project  = var.project_id
  name     = "${var.project_id}-uploads"
  location = var.region

  uniform_bucket_level_access = true

  # Images referenced by published posts must not vanish because of a
  # `terraform destroy`; emptying this bucket has to be a conscious act.
  force_destroy = false

  # Recover from an accidental overwrite or delete in the admin UI.
  versioning {
    enabled = true
  }

  # Noncurrent versions are only there for accident recovery, not archival.
  lifecycle_rule {
    condition {
      num_newer_versions = 3
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }

  # The admin UI uploads directly from the browser, so the bucket has to
  # accept cross-origin requests from the site itself.
  cors {
    origin          = compact([var.site_base_url, "http://localhost:5173", "http://localhost:8080"])
    method          = ["GET", "HEAD", "PUT", "POST"]
    response_header = ["Content-Type", "Authorization", "Content-Length"]
    max_age_seconds = 3600
  }

  depends_on = [google_project_service.apis]
}

# Post images are public by definition — they are embedded in pages meant to
# be crawled and shared. This grants read-only access to the objects; it does
# NOT allow listing the bucket or writing to it.
#
# The consequence to be aware of: anything uploaded here is world-readable
# the moment it lands, including a draft's images before the post is
# published. Do not put anything private in this bucket.
resource "google_storage_bucket_iam_member" "uploads_public_read" {
  bucket = google_storage_bucket.uploads.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}
