resource "google_firestore_database" "default" {
  project = var.project_id
  name    = "(default)"

  # Both of these are permanent. A Firestore database cannot change its
  # location or its mode after creation, and it cannot be moved to another
  # project — getting them wrong means recreating and migrating.
  location_id = var.region
  type        = "FIRESTORE_NATIVE"

  # Guards against a stray `terraform destroy` taking the content with it.
  # Deleting the database has to be a deliberate two-step: flip this to
  # ABANDON, apply, then destroy.
  delete_protection_state = "DELETE_PROTECTION_ENABLED"

  depends_on = [google_project_service.apis]
}

# Composite indexes for the queries in docs/FIRESTORE.md. Firestore creates
# single-field indexes automatically, but anything combining an equality
# filter with a different ordering field needs one of these declared.
#
# Without them the query fails at runtime with an error containing a console
# link — convenient in development, an outage in production. Declaring them
# here means they exist before the first request.

# Home page and /posts, /notes: published items of one kind, newest first.
resource "google_firestore_index" "posts_by_kind" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "posts"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }
  fields {
    field_path = "kind"
    order      = "ASCENDING"
  }
  fields {
    field_path = "publishedAt"
    order      = "DESCENDING"
  }
}

# Same, but pinned items float to the top.
resource "google_firestore_index" "posts_pinned" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "posts"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }
  fields {
    field_path = "kind"
    order      = "ASCENDING"
  }
  fields {
    field_path = "pinned"
    order      = "DESCENDING"
  }
  fields {
    field_path = "publishedAt"
    order      = "DESCENDING"
  }
}

# Tag pages: array-contains on tagSlugs, newest first.
resource "google_firestore_index" "posts_by_tag" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "posts"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }
  fields {
    field_path   = "tagSlugs"
    array_config = "CONTAINS"
  }
  fields {
    field_path = "publishedAt"
    order      = "DESCENDING"
  }
}

# Admin list view: everything of one status, most recently edited first.
resource "google_firestore_index" "posts_admin" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "posts"

  fields {
    field_path = "status"
    order      = "ASCENDING"
  }
  fields {
    field_path = "updatedAt"
    order      = "DESCENDING"
  }
}
