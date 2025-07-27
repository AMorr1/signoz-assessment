package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// CartItem represents an item in a user's shopping cart
type CartItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// Cart represents a user's shopping cart
type Cart struct {
	UserID string     `json:"user_id"`
	Items  []CartItem `json:"items"`
	mutex  sync.RWMutex
}

// CartService manages shopping carts with OpenTelemetry metrics
type CartService struct {
	carts map[string]*Cart
	mutex sync.RWMutex

	// OpenTelemetry Metrics
	errorCounter   metric.Int64Counter         // Counter: tracks error requests
	requestLatency metric.Float64Histogram     // Histogram: measures request latency
	cartItemsGauge metric.Int64ObservableGauge // Gauge: tracks cart items count

	// Additional metrics for comprehensive monitoring
	requestCounter metric.Int64Counter         // Counter: total requests
	activeUsers    metric.Int64ObservableGauge // Gauge: active users count
}

// MetricsServer wraps the CartService with HTTP handlers
type MetricsServer struct {
	service *CartService
	server  *http.Server
}

// NewCartService creates a new CartService with OpenTelemetry metrics
func NewCartService() (*CartService, error) {
	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("shopping-cart-service"),
			semconv.ServiceVersion("1.0.0"),
			semconv.ServiceInstanceID("instance-1"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithInterval(5*time.Second), // Collection interval
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Get meter
	meter := otel.Meter("shopping-cart-service")

	// Initialize service
	service := &CartService{
		carts: make(map[string]*Cart),
	}

	// Create Counter metric for error requests
	service.errorCounter, err = meter.Int64Counter(
		"http_requests_errors_total",
		metric.WithDescription("Total number of HTTP error requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	// Create Counter metric for total requests
	service.requestCounter, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request counter: %w", err)
	}

	// Create Histogram metric for request latency
	service.requestLatency, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create latency histogram: %w", err)
	}

	// Create Observable Gauge for cart items count
	service.cartItemsGauge, err = meter.Int64ObservableGauge(
		"cart_items_total",
		metric.WithDescription("Total number of items in user carts"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cart items gauge: %w", err)
	}

	// Create Observable Gauge for active users
	service.activeUsers, err = meter.Int64ObservableGauge(
		"active_users_total",
		metric.WithDescription("Total number of active users with carts"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active users gauge: %w", err)
	}

	// Register callback for observable gauges
	_, err = meter.RegisterCallback(
		service.observeCartMetrics,
		service.cartItemsGauge,
		service.activeUsers,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register callback: %w", err)
	}

	return service, nil
}

// observeCartMetrics collects gauge metrics
func (cs *CartService) observeCartMetrics(ctx context.Context, observer metric.Observer) error {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// Count total items across all carts
	totalItems := int64(0)
	for _, cart := range cs.carts {
		cart.mutex.RLock()
		for _, item := range cart.Items {
			totalItems += int64(item.Quantity)
		}
		cart.mutex.RUnlock()
	}

	// Observe metrics
	observer.ObserveInt64(cs.cartItemsGauge, totalItems)
	observer.ObserveInt64(cs.activeUsers, int64(len(cs.carts)))

	return nil
}

// recordError increments the error counter with context
func (cs *CartService) recordError(ctx context.Context, errorType, endpoint string, statusCode int) {
	cs.errorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("error_type", errorType),
			attribute.String("endpoint", endpoint),
			attribute.Int("status_code", statusCode),
		),
	)
}

// recordRequest increments the request counter
func (cs *CartService) recordRequest(ctx context.Context, method, endpoint string, statusCode int) {
	cs.requestCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", endpoint),
			attribute.Int("status_code", statusCode),
		),
	)
}

// recordLatency records request latency
func (cs *CartService) recordLatency(ctx context.Context, duration time.Duration, method, endpoint string, statusCode int) {
	cs.requestLatency.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", endpoint),
			attribute.Int("status_code", statusCode),
		),
	)
}

// AddToCart adds an item to a user's cart
func (cs *CartService) AddToCart(ctx context.Context, userID string, item CartItem) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cart, exists := cs.carts[userID]
	if !exists {
		cart = &Cart{
			UserID: userID,
			Items:  []CartItem{},
		}
		cs.carts[userID] = cart
	}

	cart.mutex.Lock()
	defer cart.mutex.Unlock()

	// Check if item already exists
	for i, existingItem := range cart.Items {
		if existingItem.ID == item.ID {
			cart.Items[i].Quantity += item.Quantity
			return nil
		}
	}

	// Add new item
	cart.Items = append(cart.Items, item)
	return nil
}

// GetCart retrieves a user's cart
func (cs *CartService) GetCart(ctx context.Context, userID string) (*Cart, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	cart, exists := cs.carts[userID]
	if !exists {
		return nil, fmt.Errorf("cart not found for user %s", userID)
	}

	cart.mutex.RLock()
	defer cart.mutex.RUnlock()

	// Create a copy to avoid race conditions
	cartCopy := &Cart{
		UserID: cart.UserID,
		Items:  make([]CartItem, len(cart.Items)),
	}
	copy(cartCopy.Items, cart.Items)

	return cartCopy, nil
}

// RemoveFromCart removes an item from a user's cart
func (cs *CartService) RemoveFromCart(ctx context.Context, userID, itemID string) error {
	cs.mutex.RLock()
	cart, exists := cs.carts[userID]
	cs.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("cart not found for user %s", userID)
	}

	cart.mutex.Lock()
	defer cart.mutex.Unlock()

	for i, item := range cart.Items {
		if item.ID == itemID {
			cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("item %s not found in cart", itemID)
}

// NewMetricsServer creates a new HTTP server with metrics endpoints
func NewMetricsServer(service *CartService, port string) *MetricsServer {
	mux := http.NewServeMux()

	server := &MetricsServer{
		service: service,
		server: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}

	// Add middleware for metrics collection
	mux.HandleFunc("/cart/add", server.withMetrics(server.handleAddToCart))
	mux.HandleFunc("/cart/get", server.withMetrics(server.handleGetCart))
	mux.HandleFunc("/cart/remove", server.withMetrics(server.handleRemoveFromCart))
	mux.HandleFunc("/health", server.withMetrics(server.handleHealth))
	mux.HandleFunc("/simulate-error", server.withMetrics(server.handleSimulateError))

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	return server
}

// withMetrics wraps HTTP handlers with metrics collection
func (ms *MetricsServer) withMetrics(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		// Create a custom response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Add random latency for demonstration
		if rand.Float32() < 0.3 { // 30% chance of additional latency
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		}

		// Call the actual handler
		handler(wrapped, r)

		// Record metrics
		duration := time.Since(start)
		statusCode := wrapped.statusCode

		ms.service.recordRequest(ctx, r.Method, r.URL.Path, statusCode)
		ms.service.recordLatency(ctx, duration, r.Method, r.URL.Path, statusCode)

		// Record error if status code indicates an error
		if statusCode >= 400 {
			errorType := "client_error"
			if statusCode >= 500 {
				errorType = "server_error"
			}
			ms.service.recordError(ctx, errorType, r.URL.Path, statusCode)
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTP Handlers

func (ms *MetricsServer) handleAddToCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string   `json:"user_id"`
		Item   CartItem `json:"item"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Item.ID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	err := ms.service.AddToCart(r.Context(), req.UserID, req.Item)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (ms *MetricsServer) handleGetCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	cart, err := ms.service.GetCart(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

func (ms *MetricsServer) handleRemoveFromCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		ItemID string `json:"item_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := ms.service.RemoveFromCart(r.Context(), req.UserID, req.ItemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (ms *MetricsServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "shopping-cart-service",
	})
}

func (ms *MetricsServer) handleSimulateError(w http.ResponseWriter, r *http.Request) {
	// Simulate different types of errors randomly
	errorTypes := []int{400, 401, 403, 404, 500, 502, 503}
	statusCode := errorTypes[rand.Intn(len(errorTypes))]

	http.Error(w, fmt.Sprintf("Simulated error with status %d", statusCode), statusCode)
}

// Start starts the HTTP server
func (ms *MetricsServer) Start() error {
	log.Printf("Starting server on %s", ms.server.Addr)
	log.Printf("Metrics available at http://localhost%s/metrics", ms.server.Addr)
	log.Printf("Health check at http://localhost%s/health", ms.server.Addr)
	return ms.server.ListenAndServe()
}

// simulateTraffic generates sample traffic for demonstration
func simulateTraffic(baseURL string) {
	go func() {
		time.Sleep(5 * time.Second) // Wait for server to start

		client := &http.Client{Timeout: 10 * time.Second}
		userIDs := []string{"user1", "user2", "user3", "user4", "user5"}

		items := []CartItem{
			{ID: "item1", Name: "Widget A", Price: 19.99, Quantity: 1},
			{ID: "item2", Name: "Widget B", Price: 29.99, Quantity: 2},
			{ID: "item3", Name: "Widget C", Price: 39.99, Quantity: 1},
			{ID: "item4", Name: "Widget D", Price: 49.99, Quantity: 3},
		}

		for {
			// Add items to random user carts
			userID := userIDs[rand.Intn(len(userIDs))]
			item := items[rand.Intn(len(items))]

			reqData := map[string]interface{}{
				"user_id": userID,
				"item":    item,
			}

			jsonData, _ := json.Marshal(reqData)
			resp, err := client.Post(baseURL+"/cart/add", "application/json",
				strings.NewReader(string(jsonData)))
			if err == nil {
				resp.Body.Close()
			}

			// Occasionally get cart
			if rand.Float32() < 0.3 {
				resp, err := client.Get(fmt.Sprintf("%s/cart/get?user_id=%s", baseURL, userID))
				if err == nil {
					resp.Body.Close()
				}
			}

			// Occasionally simulate errors
			if rand.Float32() < 0.1 {
				resp, err := client.Get(baseURL + "/simulate-error")
				if err == nil {
					resp.Body.Close()
				}
			}

			// Health check
			if rand.Float32() < 0.2 {
				resp, err := client.Get(baseURL + "/health")
				if err == nil {
					resp.Body.Close()
				}
			}

			time.Sleep(time.Duration(rand.Intn(1000)+500) * time.Millisecond)
		}
	}()
}

func main() {
	// Create cart service with OpenTelemetry metrics
	service, err := NewCartService()
	if err != nil {
		log.Fatalf("Failed to create cart service: %v", err)
	}

	// Create HTTP server
	server := NewMetricsServer(service, "8080")

	// Start traffic simulation
	simulateTraffic("http://localhost:8080")

	// Start server
	log.Fatal(server.Start())
}
