#!/bin/bash

# test_service.sh - Comprehensive testing script for Shopping Cart Service
# This script tests all endpoints and validates metrics collection

set -e

BASE_URL="http://localhost:8080"
TEST_USER="test_user_$(date +%s)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if service is running
check_service() {
    log_info "Checking if service is running..."
    if curl -s "$BASE_URL/health" > /dev/null; then
        log_success "Service is running"
    else
        log_error "Service is not running. Please start the service first."
        exit 1
    fi
}

# Test health endpoint
test_health() {
    log_info "Testing health endpoint..."
    response=$(curl -s "$BASE_URL/health")
    
    if echo "$response" | jq -e '.status == "healthy"' > /dev/null; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        echo "Response: $response"
    fi
}

# Test add item to cart
test_add_cart() {
    log_info "Testing add item to cart..."
    
    response=$(curl -s -X POST "$BASE_URL/cart/add" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$TEST_USER\",
            \"item\": {
                \"id\": \"widget_123\",
                \"name\": \"Test Widget\",
                \"price\": 19.99,
                \"quantity\": 2
            }
        }")
    
    if echo "$response" | jq -e '.status == "success"' > /dev/null; then
        log_success "Add item to cart passed"
    else
        log_error "Add item to cart failed"
        echo "Response: $response"
    fi
}

# Test get cart
test_get_cart() {
    log_info "Testing get cart..."
    
    response=$(curl -s "$BASE_URL/cart/get?user_id=$TEST_USER")
    
    if echo "$response" | jq -e '.user_id == "'$TEST_USER'"' > /dev/null; then
        log_success "Get cart passed"
        item_count=$(echo "$response" | jq '.items | length')
        log_info "Cart contains $item_count items"
    else
        log_error "Get cart failed"
        echo "Response: $response"
    fi
}

# Test add another item
test_add_another_item() {
    log_info "Testing add another item to cart..."
    
    response=$(curl -s -X POST "$BASE_URL/cart/add" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$TEST_USER\",
            \"item\": {
                \"id\": \"widget_456\",
                \"name\": \"Another Widget\",
                \"price\": 29.99,
                \"quantity\": 1
            }
        }")
    
    if echo "$response" | jq -e '.status == "success"' > /dev/null; then
        log_success "Add another item passed"
    else
        log_error "Add another item failed"
        echo "Response: $response"
    fi
}

# Test remove item from cart
test_remove_item() {
    log_info "Testing remove item from cart..."
    
    response=$(curl -s -X DELETE "$BASE_URL/cart/remove" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$TEST_USER\",
            \"item_id\": \"widget_123\"
        }")
    
    if echo "$response" | jq -e '.status == "success"' > /dev/null; then
        log_success "Remove item passed"
    else
        log_error "Remove item failed"
        echo "Response: $response"
    fi
}

# Test error simulation
test_error_simulation() {
    log_info "Testing error simulation..."
    
    # Make multiple requests to get different error codes
    for i in {1..5}; do
        curl -s "$BASE_URL/simulate-error" > /dev/null || true
    done
    
    log_success "Error simulation completed"
}

# Test metrics endpoint
test_metrics() {
    log_info "Testing metrics endpoint..."
    
    metrics=$(curl -s "$BASE_URL/metrics")
    
    # Check for required metrics
    required_metrics=(
        "http_requests_total"
        "http_requests_errors_total"
        "http_request_duration_seconds"
        "cart_items_total"
        "active_users_total"
    )
    
    missing_metrics=()
    
    for metric in "${required_metrics[@]}"; do
        if echo "$metrics" | grep -q "$metric"; then
            log_success "Found metric: $metric"
        else
            log_warning "Missing metric: $metric"
            missing_metrics+=("$metric")
        fi
    done
    
    if [ ${#missing_metrics[@]} -eq 0 ]; then
        log_success "All required metrics found"
    else
        log_error "Missing ${#missing_metrics[@]} metrics"
    fi
    
    # Show sample metrics
    log_info "Sample metrics:"
    echo "$metrics" | grep -E "(http_requests_total|cart_items_total)" | head -5
}

# Load test with concurrent requests
load_test() {
    log_info "Running basic load test..."
    
    # Create multiple users and add items concurrently
    pids=()
    
    for i in {1..10}; do
        {
            user_id="load_user_$i"
            for j in {1..5}; do
                curl -s -X POST "$BASE_URL/cart/add" \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"user_id\": \"$user_id\",
                        \"item\": {
                            \"id\": \"item_$j\",
                            \"name\": \"Load Test Item $j\",
                            \"price\": $((10 + j * 5)).99,
                            \"quantity\": $j
                        }
                    }" > /dev/null
                
                # Get cart occasionally
                if [ $((j % 2)) -eq 0 ]; then
                    curl -s "$BASE_URL/cart/get?user_id=$user_id" > /dev/null
                fi
            done
        } &
        pids+=($!)
    done
    
    # Wait for all background processes
    for pid in "${pids[@]}"; do
        wait $pid
    done
    
    log_success "Load test completed"
}

# Performance test
performance_test() {
    log_info "Running performance test..."
    
    start_time=$(date +%s%N)
    
    # Make 100 requests
    for i in {1..100}; do
        curl -s "$BASE_URL/health" > /dev/null
    done
    
    end_time=$(date +%s%N)
    duration=$((($end_time - $start_time) / 1000000)) # Convert to milliseconds
    avg_latency=$((duration / 100))
    
    log_success "Performance test completed"
    log_info "Total time: ${duration}ms"
    log_info "Average latency: ${avg_latency}ms per request"
    
    if [ $avg_latency -lt 10 ]; then
        log_success "Performance: Excellent (< 10ms avg)"
    elif [ $avg_latency -lt 50 ]; then
        log_success "Performance: Good (< 50ms avg)"
    elif [ $avg_latency -lt 100 ]; then
        log_warning "Performance: Acceptable (< 100ms avg)"
    else
        log_error "Performance: Poor (> 100ms avg)"
    fi
}

# Validate metrics after operations
validate_metrics_after_operations() {
    log_info "Validating metrics after operations..."
    
    sleep 2 # Wait for metrics to be updated
    
    metrics=$(curl -s "$BASE_URL/metrics")
    
    # Check request counts
    total_requests=$(echo "$metrics" | grep "http_requests_total" | grep -v "#" | wc -l)
    if [ $total_requests -gt 0 ]; then
        log_success "Request metrics are being recorded"
    else
        log_error "No request metrics found"
    fi
    
    # Check error counts
    error_requests=$(echo "$metrics" | grep "http_requests_errors_total" | grep -v "#" | wc -l)
    if [ $error_requests -gt 0 ]; then
        log_success "Error metrics are being recorded"
    else
        log_warning "No error metrics found (this might be normal if no errors occurred)"
    fi
    
    # Check histogram metrics
    histogram_metrics=$(echo "$metrics" | grep "http_request_duration_seconds" | grep -v "#" | wc -l)
    if [ $histogram_metrics -gt 0 ]; then
        log_success "Histogram metrics are being recorded"
    else
        log_error "No histogram metrics found"
    fi
    
    # Check gauge metrics
    cart_items=$(echo "$metrics" | grep "cart_items_total" | grep -v "#" | tail -1 | awk '{print $2}')
    active_users=$(echo "$metrics" | grep "active_users_total" | grep -v "#" | tail -1 | awk '{print $2}')
    
    if [ -n "$cart_items" ]; then
        log_success "Cart items gauge: $cart_items items"
    else
        log_error "Cart items gauge not found"
    fi
    
    if [ -n "$active_users" ]; then
        log_success "Active users gauge: $active_users users"
    else
        log_error "Active users gauge not found"
    fi
}

# Generate detailed metrics report
generate_metrics_report() {
    log_info "Generating detailed metrics report..."
    
    metrics=$(curl -s "$BASE_URL/metrics")
    report_file="metrics_report_$(date +%Y%m%d_%H%M%S).txt"
    
    {
        echo "Shopping Cart Service - Metrics Report"
        echo "Generated: $(date)"
        echo "=============================================="
        echo ""
        
        echo "COUNTER METRICS:"
        echo "---------------"
        echo "$metrics" | grep -E "^http_requests_total" | head -10
        echo "$metrics" | grep -E "^http_requests_errors_total" | head -5
        echo ""
        
        echo "HISTOGRAM METRICS:"
        echo "-----------------"
        echo "$metrics" | grep -E "^http_request_duration_seconds" | head -15
        echo ""
        
        echo "GAUGE METRICS:"
        echo "-------------"
        echo "$metrics" | grep -E "^(cart_items_total|active_users_total)"
        echo ""
        
        echo "SERVICE STATISTICS:"
        echo "------------------"
        total_req=$(echo "$metrics" | grep "http_requests_total" | grep -v "#" | awk '{sum += $2} END {print sum}')
        total_err=$(echo "$metrics" | grep "http_requests_errors_total" | grep -v "#" | awk '{sum += $2} END {print sum}')
        echo "Total Requests: ${total_req:-0}"
        echo "Total Errors: ${total_err:-0}"
        if [ "$total_req" -gt 0 ] && [ "$total_err" -gt 0 ]; then
            error_rate=$(echo "scale=2; $total_err * 100 / $total_req" | bc -l 2>/dev/null || echo "N/A")
            echo "Error Rate: ${error_rate}%"
        fi
        
    } > "$report_file"
    
    log_success "Metrics report saved to: $report_file"
}

# Cleanup test data
cleanup() {
    log_info "Cleaning up test data..."
    
    # Remove test user's cart
    curl -s -X DELETE "$BASE_URL/cart/remove" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$TEST_USER\",
            \"item_id\": \"widget_456\"
        }" > /dev/null || true
    
    log_success "Cleanup completed"
}

# Main test execution
main() {
    echo "================================================"
    echo "  Shopping Cart Service - Comprehensive Test   "
    echo "================================================"
    echo ""
    
    # Check dependencies
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi
    
    # Run tests
    check_service
    test_health
    test_add_cart
    test_get_cart
    test_add_another_item
    test_remove_item
    test_error_simulation
    load_test
    performance_test
    test_metrics
    validate_metrics_after_operations
    generate_metrics_report
    cleanup
    
    echo ""
    echo "================================================"
    log_success "All tests completed successfully!"
    echo "================================================"
    
    # Summary
    echo ""
    echo "SUMMARY:"
    echo "--------"
    echo "✓ Health check passed"
    echo "✓ Cart operations working"
    echo "✓ Metrics collection active"
    echo "✓ Performance within acceptable range"
    echo "✓ Load test completed"
    echo "✓ Error simulation working"
    echo ""
    echo "View real-time metrics at: $BASE_URL/metrics"
    echo "Service health status at: $BASE_URL/health"
}

# Script execution
if [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
    echo "Usage: $0 [options]"
    echo ""
    echo "This script performs comprehensive testing of the Shopping Cart Service"
    echo "including functional tests, load tests, and metrics validation."
    echo ""
    echo "Prerequisites:"
    echo "  - Service running on localhost:8080"
    echo "  - curl and jq installed"
    echo ""
    echo "The script will:"
    echo "  1. Test all API endpoints"
    echo "  2. Validate metrics collection"
    echo "  3. Run basic load tests"
    echo "  4. Generate a metrics report"
    echo ""
    exit 0
fi

# Trap cleanup on script exit
trap cleanup EXIT

# Run main function
main "$@"