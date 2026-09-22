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
# be crawled and shared. So anonymous GET on a known object URL has to work.
#
# The role here is legacyObjectReader, NOT objectViewer, and the difference
# matters: objectViewer includes storage.objects.list, which lets anyone
# enumerate the whole bucket. That turns "you can fetch an image if you know
# its URL" into "you can see every file that was ever uploaded" — including
# images attached to unpublished drafts, and any file uploaded by mistake.
#
# legacyObjectReader grants storage.objects.get only. Objects stay publicly
# fetchable by URL; the listing endpoint returns 403.
#
# Verify with:
#   curl https://storage.googleapis.com/storage/v1/b/<bucket>/o   # want 403
#
# Still true regardless of the role: anything in this bucket is world-readable
# the moment it lands. Do not put anything private here.
resource "google_storage_bucket_iam_member" "uploads_public_read" {
  bucket = google_storage_bucket.uploads.name
  role   = "roles/storage.legacyObjectReader"
  member = "allUsers"
}
