package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events/auth"
)

// RBACManager provides advanced role-based access control
type RBACManager struct {
	roles       map[string]*Role
	permissions map[string]*Permission
	policies    map[string]*Policy
	mutex       sync.RWMutex
	logger      Logger
}

// Role represents a role in the RBAC system
type Role struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Permissions []string               `json:"permissions"`
	Inherits    []string               `json:"inherits"`     // Roles this role inherits from
	Metadata    map[string]interface{} `json:"metadata"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Permission represents a permission in the RBAC system
type Permission struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Conditions  []string               `json:"conditions"`   // Conditional logic
	Metadata    map[string]interface{} `json:"metadata"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Policy represents an access control policy
type Policy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Rules       []PolicyRule           `json:"rules"`
	Effect      PolicyEffect           `json:"effect"`       // Allow or Deny
	Metadata    map[string]interface{} `json:"metadata"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PolicyRule defines a specific access rule
type PolicyRule struct {
	Resource   string            `json:"resource"`
	Action     string            `json:"action"`
	Conditions map[string]string `json:"conditions"`
}

// PolicyEffect defines whether a policy allows or denies access
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "allow"
	PolicyEffectDeny  PolicyEffect = "deny"
)

// AuthorizationResult represents the result of an authorization check
type AuthorizationResult struct {
	Allowed     bool                   `json:"allowed"`
	Reason      string                 `json:"reason"`
	AppliedRule string                 `json:"applied_rule,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager() *RBACManager {
	manager := &RBACManager{
		roles:       make(map[string]*Role),
		permissions: make(map[string]*Permission),
		policies:    make(map[string]*Policy),
		logger:      &SimpleLogger{},
	}
	
	// Initialize with default roles and permissions
	manager.initializeDefaults()
	
	return manager
}

// SetLogger sets a custom logger
func (r *RBACManager) SetLogger(logger Logger) {
	r.logger = logger
}

// initializeDefaults sets up default roles and permissions
func (r *RBACManager) initializeDefaults() {
	// Default permissions
	permissions := []*Permission{
		{
			ID:          "read_events",
			Name:        "Read Events",
			Description: "Read access to events",
			Resource:    "events",
			Action:      "read",
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "write_events",
			Name:        "Write Events",
			Description: "Write access to events",
			Resource:    "events",
			Action:      "write",
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "admin_events",
			Name:        "Admin Events",
			Description: "Administrative access to events",
			Resource:    "events",
			Action:      "*",
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "read_users",
			Name:        "Read Users",
			Description: "Read access to user data",
			Resource:    "users",
			Action:      "read",
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "manage_users",
			Name:        "Manage Users",
			Description: "Full access to user management",
			Resource:    "users",
			Action:      "*",
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	for _, perm := range permissions {
		r.permissions[perm.ID] = perm
	}
	
	// Default roles
	roles := []*Role{
		{
			ID:          "viewer",
			Name:        "Viewer",
			Description: "Read-only access",
			Permissions: []string{"read_events", "read_users"},
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "editor",
			Name:        "Editor",
			Description: "Read and write access",
			Permissions: []string{"read_events", "write_events", "read_users"},
			Inherits:    []string{"viewer"},
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "admin",
			Name:        "Administrator",
			Description: "Full administrative access",
			Permissions: []string{"admin_events", "manage_users"},
			Inherits:    []string{"editor"},
			Active:      true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	for _, role := range roles {
		r.roles[role.ID] = role
	}
	
	// Default policies
	policies := []*Policy{
		{
			ID:          "default_allow",
			Name:        "Default Allow Policy",
			Description: "Default policy for authenticated users",
			Rules: []PolicyRule{
				{
					Resource: "events",
					Action:   "read",
				},
			},
			Effect:    PolicyEffectAllow,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "admin_all_access",
			Name:        "Admin All Access",
			Description: "Administrators have access to everything",
			Rules: []PolicyRule{
				{
					Resource: "*",
					Action:   "*",
				},
			},
			Effect:    PolicyEffectAllow,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	
	for _, policy := range policies {
		r.policies[policy.ID] = policy
	}
}

// CheckAccess performs comprehensive access control checks
func (r *RBACManager) CheckAccess(ctx context.Context, authCtx *auth.AuthContext, resource, action string) *AuthorizationResult {
	if authCtx == nil {
		return &AuthorizationResult{
			Allowed: false,
			Reason:  "No authentication context provided",
		}
	}
	
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// Log the access check
	r.logger.Debug("Checking access",
		LogField{Key: "user", Value: authCtx.UserID},
		LogField{Key: "resource", Value: resource},
		LogField{Key: "action", Value: action})
	
	// 1. Check explicit denial policies first
	if result := r.checkPolicies(authCtx, resource, action, PolicyEffectDeny); result.Allowed {
		return &AuthorizationResult{
			Allowed:     false,
			Reason:      "Access explicitly denied by policy",
			AppliedRule: result.AppliedRule,
		}
	}
	
	// 2. Check role-based permissions
	if result := r.checkRolePermissions(authCtx, resource, action); result.Allowed {
		return result
	}
	
	// 3. Check allow policies
	if result := r.checkPolicies(authCtx, resource, action, PolicyEffectAllow); result.Allowed {
		return result
	}
	
	// 4. Default deny
	return &AuthorizationResult{
		Allowed: false,
		Reason:  "No matching permissions or policies found",
	}
}

// checkRolePermissions checks role-based permissions
func (r *RBACManager) checkRolePermissions(authCtx *auth.AuthContext, resource, action string) *AuthorizationResult {
	// Get all effective permissions for the user's roles
	effectivePermissions := r.getEffectivePermissions(authCtx.Roles)
	
	// Check each permission
	for permID := range effectivePermissions {
		perm, exists := r.permissions[permID]
		if !exists || !perm.Active {
			continue
		}
		
		if r.matchesPermission(perm, resource, action) {
			return &AuthorizationResult{
				Allowed:     true,
				Reason:      fmt.Sprintf("Granted by permission '%s'", perm.Name),
				AppliedRule: permID,
				Metadata: map[string]interface{}{
					"permission": perm,
				},
			}
		}
	}
	
	return &AuthorizationResult{Allowed: false}
}

// checkPolicies checks policy-based access control
func (r *RBACManager) checkPolicies(authCtx *auth.AuthContext, resource, action string, effect PolicyEffect) *AuthorizationResult {
	for policyID, policy := range r.policies {
		if !policy.Active || policy.Effect != effect {
			continue
		}
		
		if r.matchesPolicy(policy, authCtx, resource, action) {
			return &AuthorizationResult{
				Allowed:     effect == PolicyEffectAllow,
				Reason:      fmt.Sprintf("Matched policy '%s'", policy.Name),
				AppliedRule: policyID,
				Metadata: map[string]interface{}{
					"policy": policy,
				},
			}
		}
	}
	
	return &AuthorizationResult{Allowed: false}
}

// getEffectivePermissions gets all permissions for roles (including inherited)
func (r *RBACManager) getEffectivePermissions(userRoles []string) map[string]bool {
	effective := make(map[string]bool)
	visited := make(map[string]bool)
	
	for _, roleID := range userRoles {
		r.collectPermissions(roleID, effective, visited)
	}
	
	return effective
}

// collectPermissions recursively collects permissions from roles and their inheritance chain
func (r *RBACManager) collectPermissions(roleID string, effective, visited map[string]bool) {
	if visited[roleID] {
		return // Prevent infinite loops
	}
	visited[roleID] = true
	
	role, exists := r.roles[roleID]
	if !exists || !role.Active {
		return
	}
	
	// Add direct permissions
	for _, permID := range role.Permissions {
		effective[permID] = true
	}
	
	// Add inherited permissions
	for _, inheritedRoleID := range role.Inherits {
		r.collectPermissions(inheritedRoleID, effective, visited)
	}
}

// matchesPermission checks if a permission matches the resource and action
func (r *RBACManager) matchesPermission(perm *Permission, resource, action string) bool {
	// Check resource match
	if perm.Resource != "*" && perm.Resource != resource {
		return false
	}
	
	// Check action match
	if perm.Action != "*" && perm.Action != action {
		return false
	}
	
	return true
}

// matchesPolicy checks if a policy matches the context and request
func (r *RBACManager) matchesPolicy(policy *Policy, authCtx *auth.AuthContext, resource, action string) bool {
	for _, rule := range policy.Rules {
		if r.matchesRule(rule, authCtx, resource, action) {
			return true
		}
	}
	return false
}

// matchesRule checks if a policy rule matches the request
func (r *RBACManager) matchesRule(rule PolicyRule, authCtx *auth.AuthContext, resource, action string) bool {
	// Check resource match
	if rule.Resource != "*" && rule.Resource != resource {
		return false
	}
	
	// Check action match
	if rule.Action != "*" && rule.Action != action {
		return false
	}
	
	// Check conditions
	return r.evaluateConditions(rule.Conditions, authCtx, resource, action)
}

// evaluateConditions evaluates conditional logic in policy rules
func (r *RBACManager) evaluateConditions(conditions map[string]string, authCtx *auth.AuthContext, resource, action string) bool {
	for key, value := range conditions {
		switch key {
		case "user_id":
			if authCtx.UserID != value {
				return false
			}
		case "role":
			if !authCtx.HasRole(value) {
				return false
			}
		case "time_range":
			if !r.checkTimeRange(value) {
				return false
			}
		case "ip_range":
			// This would require access to the request context
			// Implementation depends on your specific needs
		default:
			r.logger.Warn("Unknown condition type", LogField{Key: "condition", Value: key})
		}
	}
	return true
}

// checkTimeRange validates time-based access conditions
func (r *RBACManager) checkTimeRange(timeRange string) bool {
	// Simple time range format: "09:00-17:00"
	parts := strings.Split(timeRange, "-")
	if len(parts) != 2 {
		return true // Invalid format, allow access
	}
	
	now := time.Now()
	currentTime := now.Format("15:04")
	
	return currentTime >= parts[0] && currentTime <= parts[1]
}

// Management methods for roles and permissions

// AddRole adds a new role to the system
func (r *RBACManager) AddRole(role *Role) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}
	
	if _, exists := r.roles[role.ID]; exists {
		return fmt.Errorf("role '%s' already exists", role.ID)
	}
	
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	role.Active = true
	
	r.roles[role.ID] = role
	
	r.logger.Info("Role added", LogField{Key: "role_id", Value: role.ID})
	return nil
}

// UpdateRole updates an existing role
func (r *RBACManager) UpdateRole(roleID string, updates *Role) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	role, exists := r.roles[roleID]
	if !exists {
		return fmt.Errorf("role '%s' not found", roleID)
	}
	
	// Update fields
	if updates.Name != "" {
		role.Name = updates.Name
	}
	if updates.Description != "" {
		role.Description = updates.Description
	}
	if updates.Permissions != nil {
		role.Permissions = updates.Permissions
	}
	if updates.Inherits != nil {
		role.Inherits = updates.Inherits
	}
	
	role.UpdatedAt = time.Now()
	
	r.logger.Info("Role updated", LogField{Key: "role_id", Value: roleID})
	return nil
}

// RemoveRole removes a role from the system
func (r *RBACManager) RemoveRole(roleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if _, exists := r.roles[roleID]; !exists {
		return fmt.Errorf("role '%s' not found", roleID)
	}
	
	delete(r.roles, roleID)
	
	r.logger.Info("Role removed", LogField{Key: "role_id", Value: roleID})
	return nil
}

// AddPermission adds a new permission to the system
func (r *RBACManager) AddPermission(permission *Permission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if permission.ID == "" {
		return fmt.Errorf("permission ID is required")
	}
	
	if _, exists := r.permissions[permission.ID]; exists {
		return fmt.Errorf("permission '%s' already exists", permission.ID)
	}
	
	permission.CreatedAt = time.Now()
	permission.UpdatedAt = time.Now()
	permission.Active = true
	
	r.permissions[permission.ID] = permission
	
	r.logger.Info("Permission added", LogField{Key: "permission_id", Value: permission.ID})
	return nil
}

// AddPolicy adds a new policy to the system
func (r *RBACManager) AddPolicy(policy *Policy) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	
	if _, exists := r.policies[policy.ID]; exists {
		return fmt.Errorf("policy '%s' already exists", policy.ID)
	}
	
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Active = true
	
	r.policies[policy.ID] = policy
	
	r.logger.Info("Policy added", LogField{Key: "policy_id", Value: policy.ID})
	return nil
}

// GetRole retrieves a role by ID
func (r *RBACManager) GetRole(roleID string) (*Role, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	role, exists := r.roles[roleID]
	if !exists {
		return nil, fmt.Errorf("role '%s' not found", roleID)
	}
	
	// Return a copy to prevent external modification
	roleCopy := *role
	return &roleCopy, nil
}

// ListRoles returns all roles
func (r *RBACManager) ListRoles() []*Role {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var roles []*Role
	for _, role := range r.roles {
		if role.Active {
			roleCopy := *role
			roles = append(roles, &roleCopy)
		}
	}
	
	return roles
}

// GetEffectiveRoles returns all effective roles for a user (including inherited)
func (r *RBACManager) GetEffectiveRoles(userRoles []string) []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	effective := make(map[string]bool)
	visited := make(map[string]bool)
	
	for _, roleID := range userRoles {
		r.collectRoles(roleID, effective, visited)
	}
	
	var result []string
	for roleID := range effective {
		result = append(result, roleID)
	}
	
	return result
}

// collectRoles recursively collects roles and their inheritance chain
func (r *RBACManager) collectRoles(roleID string, effective, visited map[string]bool) {
	if visited[roleID] {
		return
	}
	visited[roleID] = true
	
	role, exists := r.roles[roleID]
	if !exists || !role.Active {
		return
	}
	
	effective[roleID] = true
	
	for _, inheritedRoleID := range role.Inherits {
		r.collectRoles(inheritedRoleID, effective, visited)
	}
}