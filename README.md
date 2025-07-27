# Shopping Cart Service with OpenTelemetry Metrics

A comprehensive Go-based shopping cart service demonstrating professional OpenTelemetry metrics implementation with Counter, Histogram, and Gauge metrics. This project serves as a complete reference implementation for building observable microservices.

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for containerized setup)
- curl and jq (for testing)

### Running Locally

```bash
# Clone the repository
git clone <repository-url>
cd shopping-cart-service

# Install dependencies
go mod tidy

# Run the service
go run main.go
```

The service will start on port 8080 with the following endpoints:
- **Application**: http://localhost:8080
- **Metrics**: http://localhost:8080/metrics
- **Health Check**: http://localhost:8080/health

### Running with Docker Compose

```bash
# Build and start all services
docker-compose up --build

# Run in background
docker-compose up -d --build
```

This starts a complete observability stack:
- **Shopping Cart Service**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin123)
- **AlertManager**: http://localhost:9093

## 📊 Metrics Implementation

### Counter Metrics
- `http_requests_total` - Total HTTP requests with method, endpoint, and status code labels
- `http_requests_errors_total` - Total HTTP error requests with error type and endpoint labels

### Histogram Metrics
- `http_request_duration_seconds` - HTTP request latency distribution with customized buckets

### Gauge Metrics
- `cart_items_total` - Current total number of items across all carts
- `active_users_total` - Current number of users with active carts

## 🔧 API Endpoints

### Cart Operations

#### Add Item to Cart
```bash
curl -X POST http://localhost:8080/cart/add \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "item": {
      "id": "widget_456",
      "name": "Premium Widget",
      "price": 29.99,
      "quantity": 2
    }
  }'
```

#### Get Cart Contents
```bash
curl "http://localhost:8080/cart/get?user_id=user123"
```

#### Remove Item from Cart
```bash
curl -X DELETE http://localhost:8080/cart/remove \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "item_id": "widget_456"
  }'
```

### Operational Endpoints

#### Health Check
```bash
curl http://localhost:8080/health
```

#### Metrics (Prometheus Format)
```bash
curl http://localhost:8080/metrics
```

#### Error Simulation (for testing)
```bash
curl http://localhost:8080/simulate-error
```

## 🧪 Testing

### Automated Testing
Run the comprehensive test suite:

```bash
# Make the test script executable
chmod +x test_service.sh

# Run all tests
./test_service.sh

# View help
./test_service.sh --help
```

The test script performs:
- ✅ Functional testing of all endpoints
- ✅ Metrics validation
- ✅ Load testing with concurrent requests
- ✅ Performance benchmarking
- ✅ Error simulation testing
- ✅ Comprehensive metrics reporting

### Manual Testing
```bash
# Basic functionality test
curl -X POST http://localhost:8080/cart/add \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test","item":{"id":"test_item","name":"Test","price":10,"quantity":1}}'

# Check metrics
curl http://localhost:8080/metrics | grep -E "(http_requests_total|cart_items_total)"
```

### Load Testing
```bash
# Using Apache Bench
ab -n 1000 -c 10 http://localhost:8080/health

# Using wrk (if installed)
wrk -t4 -c100 -d30s --latency http://localhost:8080/health
```

## 📁 Project Structure

```
signoz-assessment/
├── main.go                 # Main application code
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── Dockerfile              # Container configuration
├── docker-compose.yml      # Complete stack setup
├── test_service.sh         # Comprehensive test suite
├── README.md              # This file
├── prometheus/
│   ├── prometheus.yml      # Prometheus configuration
│   └── rules/             # Alerting rules
├── grafana/
│   ├── provisioning/      # Grafana datasources and dashboards
│   └── dashboards/        # Custom dashboards
└── docs/
    └── comprehensive_guide.md  # Detailed implementation guide
```

## 🏗️ Architecture

### Service Components
- **CartService**: Core business logic with metrics instrumentation
- **MetricsServer**: HTTP server with metrics middleware
- **OpenTelemetry SDK**: Metrics collection and export
- **Prometheus Exporter**: Metrics exposure in Prometheus format

### Concurrency Design
- **Thread-safe Operations**: All cart operations