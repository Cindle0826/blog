terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }
  # Deliberately no backend block — this manages the bucket that the main
  # config's GCS backend depends on, so it has to keep its own state
  # somewhere that isn't that bucket. Local state is fine here: it's one
  # resource, applied once, by one person.
}

provider "google" {
  project = var.project_id
  region  = var.region
}
