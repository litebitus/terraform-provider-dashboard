data "dashboard_render" "mock_service" {
  spec  = file("${path.module}/../../dashboards/mock-service.yml")
  owner = "mock-owner"
  name  = "mock-service"
}

resource "aws_s3_object" "mock_service_dashboard" {
  bucket       = "mock-config-bucket"
  key          = "grafana/dashboards/mock-owner/mock-service.json"
  content_type = "application/json"
  content      = data.dashboard_render.mock_service.json
}
