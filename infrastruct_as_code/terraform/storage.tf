// Private bucket holding post images.
//
// Nothing outside the project can reach it. Readers fetch images from
// https://<site>/images/<path>, which Firebase Hosting's CDN serves and, on a
// miss, Cloud Run fills by reading the object with the blog-server identity.
//
// Serving straight from storage.googleapis.com would have been simpler, but
// it loses on both of this project's top priorities:
//
//   SEO   Google Images attributes a picture to the domain hosting it, so
//         every image would build storage.googleapis.com's presence instead
//         of the blog's.
//   Cost  Cloud Storage bills internet egress per GB, while Firebase Hosting
//         includes a monthly transfer allowance the CDN serves from.
//
// It also bakes the storage vendor into every published post: the object URL
// ends up inside the markdown, so switching later means rewriting content.
// Going through the site's own domain keeps that an implementation detail.
resource "google_storage_bucket" "uploads" {
  project  = var.project_id
  name     = "${var.project_id}-uploads"
  location = var.region

  uniform_bucket_level_access = true

  # Public access prevention is enforced at the bucket level, so a future
  # `gsutil iam ch allUsers:objectViewer` is rejected outright rather than
  # quietly making every image enumerable.
  public_access_prevention = "enforced"

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

  # No CORS rules: the browser never talks to this bucket. Uploads go to
  # POST /api/uploads on the Go server as multipart form data, and reads go
  # through GET /images/*. Both are same-origin.

  depends_on = [google_project_service.apis]
}

# Nothing grants allUsers anything here. The only principal with access is the
# Cloud Run runtime identity, via google_storage_bucket_iam_member.server_uploads
# in iam.tf.
