variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "cindle-blog"
}

variable "region" {
  description = "GCP region for the state bucket"
  type        = string
  default     = "asia-east1"
}
