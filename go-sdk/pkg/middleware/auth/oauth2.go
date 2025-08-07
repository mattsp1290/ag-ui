package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth2Provider implements OAuth 2.0 authentication
type OAuth2Provider struct {
	config     *OAuth2Config
	cache      TokenCache
	auditor    AuditLogger
	httpClient *http.Client
}

// OAuth2TokenInfo represents OAuth 2.0 token information
type OAuth2TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int64     `json:"expires_in"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	IssuedAt     time.Time `json:"issued_at"`
}

// OAuth2UserInfo represents OAuth 2.0 user information
type OAuth2UserInfo struct {
	ID            string            `json:"id"`
	Username      string            `json:"username,omitempty"`
	Email         string            `json:"email,omitempty"`
	Name          string            `json:"name,omitempty"`
	GivenName     string            `json:"given_name,omitempty"`
	FamilyName    string            `json:"family_name,omitempty"`
	Picture       string            `json:"picture,omitempty"`
	Locale        string            `json:"locale,omitempty"`
	EmailVerified bool              `json:"email_verified,omitempty"`
	Roles         []string          `json:"roles,omitempty"`
	Groups        []string          `json:"groups,omitempty"`
	Permissions   []string          `json:"permissions,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// NewOAuth2Provider creates a new OAuth 2.0 authentication provider
func NewOAuth2Provider(config *OAuth2Config, cache TokenCache, auditor AuditLogger) *OAuth2Provider {
	if config == nil {
		return nil
	}

	return &OAuth2Provider{
		config:     config,
		cache:      cache,
		auditor:    auditor,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider name
func (o *OAuth2Provider) Name() string {
	return "oauth2"
}

// Authenticate validates OAuth 2.0 token and returns authentication context
func (o *OAuth2Provider) Authenticate(ctx context.Context, credentials *Credentials) (*AuthContext, error) {
	if credentials == nil || credentials.Token == "" {
		return nil, fmt.Errorf("OAuth 2.0 access token is required")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("oauth2:%s", credentials.Token)
	if o.cache != nil {
		if cached, err := o.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			return o.credentialsToAuthContext(cached), nil
		}
	}

	// Validate token and get user info
	userInfo, err := o.getUserInfo(ctx, credentials.Token)
	if err != nil {
		if o.auditor != nil {
			o.auditor.LogAuthFailure(ctx, fmt.Sprintf("OAuth2 token validation failed: %v", err), map[string]any{
				"token_prefix": o.tokenPrefix(credentials.Token),
			})
		}
		return nil, fmt.Errorf("OAuth2 token validation failed: %w", err)
	}

	// Create auth context
	authCtx := &AuthContext{
		UserID:      userInfo.ID,
		Username:    userInfo.Username,
		Roles:       userInfo.Roles,
		Permissions: userInfo.Permissions,
		Metadata:    userInfo.Metadata,
		AuthMethod:  "oauth2",
		Timestamp:   time.Now(),
	}

	if userInfo.Email != "" {
		authCtx.Username = userInfo.Email
	}

	// Cache the credentials
	if o.cache != nil {
		cacheCredentials := &Credentials{
			Type:     "oauth2",
			Token:    credentials.Token,
			Claims:   map[string]any{"user_info": userInfo},
			Subject:  userInfo.ID,
			Metadata: userInfo.Metadata,
		}

		// Cache for 5 minutes by default
		_ = o.cache.Set(ctx, cacheKey, cacheCredentials, 5*time.Minute)
	}

	// Log successful authentication
	if o.auditor != nil {
		o.auditor.LogAuthSuccess(ctx, authCtx, map[string]any{
			"token_prefix": o.tokenPrefix(credentials.Token),
			"user_id":      userInfo.ID,
			"email":        userInfo.Email,
		})
	}

	return authCtx, nil
}

// Validate validates an existing authentication context
func (o *OAuth2Provider) Validate(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return fmt.Errorf("authentication context is required")
	}

	if authCtx.AuthMethod != "oauth2" {
		return fmt.Errorf("invalid authentication method: %s", authCtx.AuthMethod)
	}

	// Check expiration
	if !authCtx.ExpiresAt.IsZero() && time.Now().After(authCtx.ExpiresAt) {
		return fmt.Errorf("authentication context has expired")
	}

	return nil
}

// Refresh attempts to refresh OAuth 2.0 token
func (o *OAuth2Provider) Refresh(ctx context.Context, credentials *Credentials) (*Credentials, error) {
	if credentials == nil || credentials.Token == "" {
		return nil, fmt.Errorf("access token is required for refresh")
	}

	// Extract refresh token from metadata
	refreshToken := ""
	if credentials.Metadata != nil {
		refreshToken = credentials.Metadata["refresh_token"]
	}

	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	// Prepare refresh request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", o.config.ClientID)
	if o.config.ClientSecret != "" {
		data.Set("client_secret", o.config.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var tokenInfo OAuth2TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Calculate expiration
	tokenInfo.IssuedAt = time.Now()
	tokenInfo.ExpiresAt = tokenInfo.IssuedAt.Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)

	refreshedCredentials := &Credentials{
		Type:      "oauth2",
		Token:     tokenInfo.AccessToken,
		ExpiresAt: tokenInfo.ExpiresAt,
		IssuedAt:  tokenInfo.IssuedAt,
		Scopes:    strings.Fields(tokenInfo.Scope),
		Metadata: map[string]string{
			"token_type": tokenInfo.TokenType,
		},
	}

	if tokenInfo.RefreshToken != "" {
		refreshedCredentials.Metadata["refresh_token"] = tokenInfo.RefreshToken
	}

	// Remove old token from cache
	if o.cache != nil {
		oldCacheKey := fmt.Sprintf("oauth2:%s", credentials.Token)
		_ = o.cache.Delete(ctx, oldCacheKey)
	}

	// Log token refresh
	if o.auditor != nil {
		authCtx := &AuthContext{
			UserID:     credentials.Subject,
			AuthMethod: "oauth2",
			Timestamp:  time.Now(),
		}
		o.auditor.LogTokenRefresh(ctx, authCtx)
	}

	return refreshedCredentials, nil
}

// Revoke revokes OAuth 2.0 token
func (o *OAuth2Provider) Revoke(ctx context.Context, credentials *Credentials) error {
	if credentials == nil || credentials.Token == "" {
		return fmt.Errorf("access token is required")
	}

	// Remove from cache
	if o.cache != nil {
		cacheKey := fmt.Sprintf("oauth2:%s", credentials.Token)
		_ = o.cache.Delete(ctx, cacheKey)
	}

	// Note: OAuth 2.0 token revocation would typically involve calling
	// the provider's revocation endpoint, but this is simplified for the interface
	
	return nil
}

// SupportedTypes returns supported credential types
func (o *OAuth2Provider) SupportedTypes() []string {
	return []string{"oauth2", "bearer"}
}

// getUserInfo retrieves user information using the access token
func (o *OAuth2Provider) getUserInfo(ctx context.Context, accessToken string) (*OAuth2UserInfo, error) {
	if o.config.UserInfoURL == "" {
		// If no user info URL is configured, return minimal user info
		return &OAuth2UserInfo{
			ID: "oauth2_user",
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", o.config.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed with status: %d", resp.StatusCode)
	}

	var userInfo OAuth2UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// credentialsToAuthContext converts credentials to auth context
func (o *OAuth2Provider) credentialsToAuthContext(credentials *Credentials) *AuthContext {
	authCtx := &AuthContext{
		UserID:     credentials.Subject,
		Claims:     credentials.Claims,
		Metadata:   credentials.Metadata,
		AuthMethod: "oauth2",
		Timestamp:  time.Now(),
		ExpiresAt:  credentials.ExpiresAt,
	}

	// Extract user info from claims if available
	if userInfoData, ok := credentials.Claims["user_info"]; ok {
		if userInfo, ok := userInfoData.(*OAuth2UserInfo); ok {
			authCtx.Username = userInfo.Username
			authCtx.Roles = userInfo.Roles
			authCtx.Permissions = userInfo.Permissions
			if userInfo.Email != "" {
				authCtx.Username = userInfo.Email
			}
		}
	}

	return authCtx
}

// tokenPrefix returns the first few characters of token for logging
func (o *OAuth2Provider) tokenPrefix(token string) string {
	if len(token) > 16 {
		return token[:16] + "..."
	}
	return token
}

// GetAuthorizationURL generates the authorization URL for OAuth 2.0 flow
func (o *OAuth2Provider) GetAuthorizationURL(state, codeChallenge string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", o.config.ClientID)
	params.Set("redirect_uri", o.config.RedirectURL)
	params.Set("state", state)
	
	if len(o.config.Scopes) > 0 {
		params.Set("scope", strings.Join(o.config.Scopes, " "))
	}
	
	if codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")
	}

	return fmt.Sprintf("%s?%s", o.config.AuthURL, params.Encode())
}

// ExchangeCodeForToken exchanges authorization code for access token
func (o *OAuth2Provider) ExchangeCodeForToken(ctx context.Context, code, codeVerifier string) (*OAuth2TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", o.config.ClientID)
	data.Set("redirect_uri", o.config.RedirectURL)
	
	if o.config.ClientSecret != "" {
		data.Set("client_secret", o.config.ClientSecret)
	}
	
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenInfo OAuth2TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Calculate expiration
	tokenInfo.IssuedAt = time.Now()
	tokenInfo.ExpiresAt = tokenInfo.IssuedAt.Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)

	return &tokenInfo, nil
}

// OAuth2Middleware implements OAuth 2.0 authentication middleware
type OAuth2Middleware struct {
	provider  *OAuth2Provider
	extractor CredentialExtractor
	enabled   bool
	priority  int
}

// NewOAuth2Middleware creates new OAuth 2.0 authentication middleware
func NewOAuth2Middleware(config *OAuth2Config, cache TokenCache, auditor AuditLogger) *OAuth2Middleware {
	provider := NewOAuth2Provider(config, cache, auditor)

	return &OAuth2Middleware{
		provider:  provider,
		extractor: NewBearerTokenExtractor(), // Reuse bearer token extractor
		enabled:   true,
		priority:  95, // High priority for authentication
	}
}

// Name returns middleware name
func (o *OAuth2Middleware) Name() string {
	return "oauth2_auth"
}

// Process processes the request through OAuth 2.0 authentication
func (o *OAuth2Middleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Extract credentials
	credentials, err := o.extractor.Extract(ctx, req.Headers, req.Body)
	if err != nil || credentials == nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
			Error:      fmt.Errorf("OAuth2 access token required: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Authenticate
	authCtx, err := o.provider.Authenticate(ctx, credentials)
	if err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
			Error:      fmt.Errorf("authentication failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add auth context to request metadata
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["auth_context"] = authCtx

	// Continue to next middleware
	return next(ctx, req)
}

// Configure configures the middleware
func (o *OAuth2Middleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		o.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		o.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (o *OAuth2Middleware) Enabled() bool {
	return o.enabled
}

// Priority returns the middleware priority
func (o *OAuth2Middleware) Priority() int {
	return o.priority
}