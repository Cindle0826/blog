terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
    # Firebase 的資源從來沒有進入 GA provider，只能用 beta。
    # 只有 firebase.tf 用到它，其餘一律走 GA provider。
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 7.0"
    }
  }

  backend "gcs" {
    bucket = "cindle-blog-tfstate"
    prefix = "blog"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region

  # Local ADC has no inherent quota project; some APIs (billing budgets
  # among them) reject requests without one. Route quota/billing to our
  # actual project instead of the gcloud CLI's own default.
  user_project_override = true
  billing_project       = var.project_id
}

provider "google-beta" {
  project = var.project_id
  region  = var.region

  user_project_override = true
  billing_project       = var.project_id
}
