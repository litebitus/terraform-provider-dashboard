resource "aws_s3_object" "mock_service_dashboard" {
  bucket       = "mock-config-bucket"
  key          = "grafana/dashboards/mock-owner/mock-service.json"
  content_type = "application/json"
  content      = provider::dashboard::render(file("${path.module}/../../dashboards/mock-service.yml"))
}

output "mock_service_dashboard_for_grafana" {
  value = provider::dashboard::render(
    file("${path.module}/../../dashboards/mock-service.yml"),
    { target_schema = "grafana" },
  )
}
