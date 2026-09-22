resource "google_storage_bucket" "tfstate" {
  project                     = var.project_id
  name                        = "${var.project_id}-tfstate"
  location                    = var.region
  uniform_bucket_level_access = true

  # Losing this bucket means losing the record of every managed resource.
  force_destroy = false

  # Terraform state is the one thing you really want a previous copy of when
  # an apply goes wrong.
  versioning {
    enabled = true
  }
}

output "bucket_name" {
  value = google_storage_bucket.tfstate.name
}
