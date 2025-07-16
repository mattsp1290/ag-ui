package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events/auth"
)

func main() {
	fmt.Println("AG-UI Authentication Middleware Example")
	fmt.Println("======================================")

	// 1. Initialize authentication provider
	provider := setupAuthProvider()

	// 2. Initialize RBAC manager
	rbacManager := setupRBACManager()

	// 3. Create middleware with different configurations
	demonstrateMiddlewareConfigurations(provider, rbacManager)

	// 4. Start HTTP server with examples
	startServer(provider, rbacManager)
}

// setupAuthProvider creates and configures an authentication provider
func setupAuthProvider() auth.AuthProvider {
	fmt.Println("\n1. Setting up Authentication Provider")
	fmt.Println("=====================================")

	// Create basic auth provider
	config := auth.DefaultAuthConfig()
	config.TokenExpiration = 1 * time.Hour
	provider := auth.NewBasicAuthProvider(config)

	// Add test users
	testUsers := []*auth.User{
		{
			ID:       "user-1",
			Username: "alice",
			Roles:    []string{"admin"},
			Permissions: []string{
				"events:*",
				"users:*",
				"admin:*",
			},
			Active: true,
			Metadata: map[string]interface{}{
				"department": "engineering",
				"api_key":    "alice-api-key-123",
			},
		},
		{
			ID:       "user-2",
			Username: "bob",
			Roles:    []string{"editor"},
			Permissions: []string{
				"events:read",
				"events:write",
				"users:read",
			},
			Active: true,
			Metadata: map[string]interface{}{
				"department": "product",
			},
		},
		{
			ID:       "user-3",
			Username: "charlie",
			Roles:    []string{"viewer"},
			Permissions: []string{
				"events:read",
				"users:read",
			},
			Active: true,
			Metadata: map[string]interface{}{
				"department": "marketing",
			},
		},
	}

	for _, user := range testUsers {
		if err := provider.AddUser(user); err != nil {
			log.Printf("Failed to add user %s: %v", user.Username, err)
			continue
		}

		// Set password (in production, use proper password management)
		if err := provider.SetUserPassword(user.Username, "password123"); err != nil {
			log.Printf("Failed to set password for %s: %v", user.Username, err)
		}

		fmt.Printf("✓ Added user: %s (roles: %v)\n", user.Username, user.Roles)
	}

	return provider
}

// setupRBACManager creates and configures an RBAC manager
func setupRBACManager() *RBACManager {
	fmt.Println("\n2. Setting up RBAC Manager")
	fmt.Println("=========================")

	rbac := NewRBACManager()

	// Add custom roles
	customRoles := []*Role{
		{
			ID:          "event_manager",
			Name:        "Event Manager",
			Description: "Specialized role for event management",
			Permissions: []string{"read_events", "write_events"},
			Inherits:    []string{"viewer"},
		},
		{
			ID:          "security_admin",
			Name:        "Security Administrator",
			Description: "Security-focused administrative role",
			Permissions: []string{"admin_events", "read_users"},
			Inherits:    []string{"editor"},
		},
	}

	for _, role := range customRoles {
		if err := rbac.AddRole(role); err != nil {
			log.Printf("Failed to add role %s: %v", role.ID, err)
		} else {
			fmt.Printf("✓ Added custom role: %s\n", role.Name)
		}
	}

	// Add custom permissions
	customPermissions := []*Permission{
		{
			ID:          "export_events",
			Name:        "Export Events",
			Description: "Permission to export event data",
			Resource:    "events",
			Action:      "export",
		},
		{
			ID:          "analytics_access",
			Name:        "Analytics Access",
			Description: "Access to analytics dashboard",
			Resource:    "analytics",
			Action:      "read",
		},
	}

	for _, perm := range customPermissions {
		if err := rbac.AddPermission(perm); err != nil {
			log.Printf("Failed to add permission %s: %v", perm.ID, err)
		} else {
			fmt.Printf("✓ Added custom permission: %s\n", perm.Name)
		}
	}

	// Add time-based policy
	timeBasedPolicy := &Policy{
		ID:          "business_hours_only",
		Name:        "Business Hours Only",
		Description: "Allow access only during business hours",
		Rules: []PolicyRule{
			{
				Resource: "analytics",
				Action:   "read",
				Conditions: map[string]string{
					"time_range": "09:00-17:00",
				},
			},
		},
		Effect: PolicyEffectAllow,
	}

	if err := rbac.AddPolicy(timeBasedPolicy); err != nil {
		log.Printf("Failed to add time-based policy: %v", err)
	} else {
		fmt.Printf("✓ Added time-based policy: %s\n", timeBasedPolicy.Name)
	}

	return rbac
}

// demonstrateMiddlewareConfigurations shows different middleware configurations
func demonstrateMiddlewareConfigurations(provider auth.AuthProvider, rbac *RBACManager) {
	fmt.Println("\n3. Middleware Configuration Examples")
	fmt.Println("====================================")

	// 1. Strict security configuration
	strictConfig := &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      false,
		RequiredRoles:       []string{"admin", "editor"},
		RequiredPermissions: []string{"events:read"},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           30, // 30 requests per minute
		CORSEnabled:         false,
	}
	fmt.Printf("✓ Strict config: Requires admin/editor role, secure errors, rate limited\n")

	// 2. Development configuration
	devConfig := &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      true,
		RequiredRoles:       []string{},
		RequiredPermissions: []string{},
		SecureErrorMode:     false, // Show detailed errors
		RateLimiting:        false,
		CORSEnabled:         true,
		AllowedOrigins:      []string{"*"},
	}
	fmt.Printf("✓ Dev config: Anonymous access, detailed errors, CORS enabled\n")

	// 3. API gateway configuration
	apiConfig := &AuthMiddlewareConfig{
		TokenHeader:         "X-API-Key",
		TokenPrefix:         "",
		AllowAnonymous:      false,
		RequiredRoles:       []string{},
		RequiredPermissions: []string{},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           100, // Higher limit for API access
		CORSEnabled:         true,
		AllowedOrigins:      []string{"https://api.example.com", "https://dashboard.example.com"},
	}
	fmt.Printf("✓ API config: API key auth, high rate limit, specific CORS origins\n")

	// Store configurations for later use
	_ = strictConfig
	_ = devConfig
	_ = apiConfig
}

// startServer starts an HTTP server with authentication examples
func startServer(provider auth.AuthProvider, rbac *RBACManager) {
	fmt.Println("\n4. Starting HTTP Server with Examples")
	fmt.Println("=====================================")

	mux := http.NewServeMux()

	// Create different middleware instances for different endpoints
	adminMiddleware := NewAuthMiddleware(provider, &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      false,
		RequiredRoles:       []string{"admin"},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           60,
	})

	editorMiddleware := NewAuthMiddleware(provider, &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      false,
		RequiredRoles:       []string{"admin", "editor"},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           120,
	})

	viewerMiddleware := NewAuthMiddleware(provider, &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      false,
		RequiredRoles:       []string{"admin", "editor", "viewer"},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           200,
	})

	// Public endpoints (no authentication required)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/login", loginHandler(provider))

	// Protected endpoints with different access levels
	mux.Handle("/admin/users", adminMiddleware.Middleware()(http.HandlerFunc(adminUsersHandler)))
	mux.Handle("/admin/system", adminMiddleware.Middleware()(http.HandlerFunc(adminSystemHandler)))

	mux.Handle("/events", editorMiddleware.Middleware()(http.HandlerFunc(eventsHandler)))
	mux.Handle("/events/create", editorMiddleware.Middleware()(http.HandlerFunc(createEventHandler)))

	mux.Handle("/dashboard", viewerMiddleware.Middleware()(http.HandlerFunc(dashboardHandler)))
	mux.Handle("/profile", viewerMiddleware.Middleware()(http.HandlerFunc(profileHandler)))

	// RBAC demo endpoints
	mux.Handle("/rbac/check", viewerMiddleware.Middleware()(http.HandlerFunc(rbacCheckHandler(rbac))))

	// Create server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		fmt.Printf("Server starting on http://localhost:8080\n")
		fmt.Println("\nEndpoints:")
		fmt.Println("  Public:")
		fmt.Println("    GET  /health              - Health check")
		fmt.Println("    POST /login               - Authentication")
		fmt.Println("  Viewer+ (any authenticated user):")
		fmt.Println("    GET  /dashboard           - User dashboard")
		fmt.Println("    GET  /profile             - User profile")
		fmt.Println("    POST /rbac/check          - RBAC access check")
		fmt.Println("  Editor+ (editor or admin):")
		fmt.Println("    GET  /events              - List events")
		fmt.Println("    POST /events/create       - Create event")
		fmt.Println("  Admin only:")
		fmt.Println("    GET  /admin/users         - User management")
		fmt.Println("    GET  /admin/system        - System administration")

		fmt.Println("\nAuthentication Examples:")
		fmt.Println("  1. Login to get token:")
		fmt.Println("     curl -X POST http://localhost:8080/login \\")
		fmt.Println("          -H 'Content-Type: application/json' \\")
		fmt.Println("          -d '{\"username\":\"alice\",\"password\":\"password123\"}'")
		fmt.Println()
		fmt.Println("  2. Use token for authenticated requests:")
		fmt.Println("     curl -H 'Authorization: Bearer <token>' http://localhost:8080/profile")
		fmt.Println()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\nShutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	fmt.Println("Server stopped gracefully")
}

// HTTP Handlers

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func loginHandler(provider auth.AuthProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var loginReq struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		// Authenticate
		creds := &auth.BasicCredentials{
			Username: loginReq.Username,
			Password: loginReq.Password,
		}

		authCtx, err := provider.Authenticate(r.Context(), creds)
		if err != nil {
			writeErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}

		response := map[string]interface{}{
			"token":        authCtx.Token,
			"user_id":      authCtx.UserID,
			"username":     authCtx.Username,
			"roles":        authCtx.Roles,
			"permissions":  authCtx.Permissions,
			"expires_at":   authCtx.ExpiresAt.Unix(),
		}

		writeJSONResponse(w, http.StatusOK, response)
	}
}

func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := GetUserID(r.Context())
	roles, _ := GetUserRoles(r.Context())

	response := map[string]interface{}{
		"message":      "Admin users endpoint accessed",
		"accessed_by":  userID,
		"user_roles":   roles,
		"endpoint":     "admin/users",
		"access_level": "admin",
		"users": []map[string]interface{}{
			{"id": "user-1", "username": "alice", "roles": []string{"admin"}},
			{"id": "user-2", "username": "bob", "roles": []string{"editor"}},
			{"id": "user-3", "username": "charlie", "roles": []string{"viewer"}},
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func adminSystemHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := GetUserID(r.Context())

	response := map[string]interface{}{
		"message":      "System administration endpoint accessed",
		"accessed_by":  userID,
		"endpoint":     "admin/system",
		"access_level": "admin",
		"system_info": map[string]interface{}{
			"uptime":        "2h 15m",
			"memory_usage":  "45%",
			"active_users":  42,
			"rate_limiting": "enabled",
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := GetUserID(r.Context())
	roles, _ := GetUserRoles(r.Context())

	response := map[string]interface{}{
		"message":      "Events endpoint accessed",
		"accessed_by":  userID,
		"user_roles":   roles,
		"endpoint":     "events",
		"access_level": "editor+",
		"events": []map[string]interface{}{
			{"id": "evt-1", "type": "user_login", "timestamp": time.Now().Unix()},
			{"id": "evt-2", "type": "data_update", "timestamp": time.Now().Unix() - 3600},
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func createEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, _ := GetUserID(r.Context())
	permissions, _ := GetUserPermissions(r.Context())

	response := map[string]interface{}{
		"message":         "Event created successfully",
		"created_by":      userID,
		"user_permissions": permissions,
		"event_id":        "evt-" + fmt.Sprintf("%d", time.Now().Unix()),
		"endpoint":        "events/create",
		"access_level":    "editor+",
	}

	writeJSONResponse(w, http.StatusCreated, response)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := GetUserID(r.Context())
	roles, _ := GetUserRoles(r.Context())
	permissions, _ := GetUserPermissions(r.Context())

	response := map[string]interface{}{
		"message":      "Dashboard accessed",
		"accessed_by":  userID,
		"user_roles":   roles,
		"endpoint":     "dashboard",
		"access_level": "viewer+",
		"dashboard_data": map[string]interface{}{
			"total_events":     156,
			"recent_activity":  23,
			"user_permissions": permissions,
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := GetUserID(r.Context())
	roles, _ := GetUserRoles(r.Context())
	permissions, _ := GetUserPermissions(r.Context())

	response := map[string]interface{}{
		"message":      "User profile accessed",
		"user_id":      userID,
		"roles":        roles,
		"permissions":  permissions,
		"endpoint":     "profile",
		"access_level": "viewer+",
		"profile_data": map[string]interface{}{
			"last_login":    time.Now().Add(-2 * time.Hour).Unix(),
			"session_count": 15,
			"preferences": map[string]interface{}{
				"theme":    "dark",
				"language": "en",
			},
		},
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func rbacCheckHandler(rbac *RBACManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var checkReq struct {
			Resource string `json:"resource"`
			Action   string `json:"action"`
		}

		if err := json.NewDecoder(r.Body).Decode(&checkReq); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		userID, _ := GetUserID(r.Context())
		roles, _ := GetUserRoles(r.Context())
		permissions, _ := GetUserPermissions(r.Context())

		// Create auth context for RBAC check
		authCtx := &auth.AuthContext{
			UserID:      userID,
			Roles:       roles,
			Permissions: permissions,
		}

		result := rbac.CheckAccess(r.Context(), authCtx, checkReq.Resource, checkReq.Action)

		response := map[string]interface{}{
			"message":          "RBAC access check completed",
			"checked_by":       userID,
			"requested_access": map[string]string{
				"resource": checkReq.Resource,
				"action":   checkReq.Action,
			},
			"result": result,
		}

		writeJSONResponse(w, http.StatusOK, response)
	}
}

// Utility functions

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Unix(),
	}
	writeJSONResponse(w, statusCode, response)
}