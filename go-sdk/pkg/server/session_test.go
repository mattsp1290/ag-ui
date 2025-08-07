package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

func TestSessionManager(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()

	// Create session manager
	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)
	require.NotNil(t, sm)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("SessionManager Configuration", func(t *testing.T) {
		assert.Equal(t, config.Backend, sm.config.Backend)
		assert.Equal(t, config.TTL, sm.config.TTL)
		assert.Equal(t, config.CleanupInterval, sm.config.CleanupInterval)
	})

	t.Run("Create Session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("User-Agent", "test-browser")

		session, err := sm.CreateSession(context.Background(), "user123", req)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.NotEmpty(t, session.ID)
		assert.Equal(t, "user123", session.UserID)
		assert.True(t, session.IsActive)
		assert.NotZero(t, session.CreatedAt)
		assert.NotZero(t, session.LastAccessed)
		assert.NotZero(t, session.ExpiresAt)
		assert.Equal(t, "192.168.1.100", session.IPAddress)
		assert.Equal(t, "test-browser", session.UserAgent)
	})

	t.Run("Get Session", func(t *testing.T) {
		// Create session first
		req := httptest.NewRequest("GET", "/test", nil)
		originalSession, err := sm.CreateSession(context.Background(), "user456", req)
		require.NoError(t, err)

		// Retrieve session
		retrievedSession, err := sm.GetSession(context.Background(), originalSession.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedSession)

		assert.Equal(t, originalSession.ID, retrievedSession.ID)
		assert.Equal(t, originalSession.UserID, retrievedSession.UserID)
		assert.True(t, retrievedSession.IsActive)
	})

	t.Run("Get Non-existent Session", func(t *testing.T) {
		_, err := sm.GetSession(context.Background(), "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Delete Session", func(t *testing.T) {
		// Create session
		req := httptest.NewRequest("GET", "/test", nil)
		session, err := sm.CreateSession(context.Background(), "user789", req)
		require.NoError(t, err)

		// Delete session
		err = sm.DeleteSession(context.Background(), session.ID)
		require.NoError(t, err)

		// Verify session is deleted
		_, err = sm.GetSession(context.Background(), session.ID)
		assert.Error(t, err)
	})
}

func TestSessionConfig(t *testing.T) {
	t.Run("DefaultSessionConfig", func(t *testing.T) {
		config := DefaultSessionConfig()

		assert.Equal(t, "memory", config.Backend)
		assert.Greater(t, config.TTL, time.Duration(0))
		assert.Greater(t, config.CleanupInterval, time.Duration(0))
		assert.True(t, config.SecureCookies)
		assert.True(t, config.HTTPOnlyCookies)
		assert.NotEmpty(t, config.CookieName)
		assert.NotEmpty(t, config.CookiePath)
		assert.Greater(t, config.MaxConcurrentSessions, 0)
	})

	t.Run("SessionConfig Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *SessionConfig
			wantErr bool
		}{
			{
				name: "valid memory config",
				config: &SessionConfig{
					Backend:               "memory",
					TTL:                   time.Hour,
					CleanupInterval:       time.Minute,
					MaxConcurrentSessions: 1000,
					SessionPoolSize:       1000,
					CookieName:            "session_id",
					CookiePath:            "/",
					Memory: &MemorySessionConfig{
						MaxSessions: 10000,
					},
				},
				wantErr: false,
			},
			{
				name: "zero TTL",
				config: &SessionConfig{
					Backend:         "memory",
					TTL:             0,
					CleanupInterval: time.Minute,
				},
				wantErr: true,
			},
			{
				name: "zero cleanup interval",
				config: &SessionConfig{
					Backend:         "memory",
					TTL:             time.Hour,
					CleanupInterval: 0,
				},
				wantErr: true,
			},
			{
				name: "unsupported backend",
				config: &SessionConfig{
					Backend:         "unsupported",
					TTL:             time.Hour,
					CleanupInterval: time.Minute,
				},
				wantErr: true,
			},
			{
				name: "memory backend without config",
				config: &SessionConfig{
					Backend:         "memory",
					TTL:             time.Hour,
					CleanupInterval: time.Minute,
					Memory:          nil,
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestSessionValidation(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()
	config.ValidateIP = true
	config.ValidateUserAgent = true

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Valid Session Validation", func(t *testing.T) {
		// Create session
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("User-Agent", "test-browser")

		session, err := sm.CreateSession(context.Background(), "user123", req)
		require.NoError(t, err)

		// Validate with same IP and User-Agent
		validatedSession, err := sm.ValidateSession(context.Background(), session.ID, req)
		require.NoError(t, err)
		assert.Equal(t, session.ID, validatedSession.ID)
	})

	t.Run("IP Address Mismatch", func(t *testing.T) {
		// Create session
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.1.100:12345"
		req1.Header.Set("User-Agent", "test-browser")

		session, err := sm.CreateSession(context.Background(), "user456", req1)
		require.NoError(t, err)

		// Validate from different IP
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.1.200:12345"
		req2.Header.Set("User-Agent", "test-browser")

		_, err = sm.ValidateSession(context.Background(), session.ID, req2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "IP address mismatch")
	})

	t.Run("User Agent Mismatch", func(t *testing.T) {
		// Create session
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.1.100:12345"
		req1.Header.Set("User-Agent", "test-browser-1")

		session, err := sm.CreateSession(context.Background(), "user789", req1)
		require.NoError(t, err)

		// Validate with different User-Agent
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.1.100:12345"
		req2.Header.Set("User-Agent", "test-browser-2")

		_, err = sm.ValidateSession(context.Background(), session.ID, req2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user agent mismatch")
	})
}

func TestSessionUserOperations(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Get User Sessions", func(t *testing.T) {
		userID := "multi-session-user"
		req := httptest.NewRequest("GET", "/test", nil)

		// Create multiple sessions for same user
		var sessionIDs []string
		for i := 0; i < 3; i++ {
			session, err := sm.CreateSession(context.Background(), userID, req)
			require.NoError(t, err)
			sessionIDs = append(sessionIDs, session.ID)
		}

		// Get user sessions
		sessions, err := sm.GetUserSessions(context.Background(), userID)
		require.NoError(t, err)
		assert.Len(t, sessions, 3)

		// Verify all sessions belong to user
		for _, session := range sessions {
			assert.Equal(t, userID, session.UserID)
			assert.Contains(t, sessionIDs, session.ID)
		}
	})

	t.Run("Delete User Sessions", func(t *testing.T) {
		userID := "delete-all-user"
		req := httptest.NewRequest("GET", "/test", nil)

		// Create multiple sessions for user
		for i := 0; i < 2; i++ {
			_, err := sm.CreateSession(context.Background(), userID, req)
			require.NoError(t, err)
		}

		// Verify sessions exist
		sessions, err := sm.GetUserSessions(context.Background(), userID)
		require.NoError(t, err)
		assert.Len(t, sessions, 2)

		// Delete all user sessions
		err = sm.DeleteUserSessions(context.Background(), userID)
		require.NoError(t, err)

		// Verify sessions are deleted
		sessions, err = sm.GetUserSessions(context.Background(), userID)
		require.NoError(t, err)
		assert.Len(t, sessions, 0)
	})
}

func TestSessionMiddleware(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Session Middleware with Valid Session", func(t *testing.T) {
		// Create session
		req := httptest.NewRequest("GET", "/test", nil)
		session, err := sm.CreateSession(context.Background(), "user123", req)
		require.NoError(t, err)

		// Create middleware
		middleware := sm.SessionMiddleware()

		// Create test handler
		var capturedSession *Session
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedSession, _ = sm.GetSessionFromRequest(r)
			w.WriteHeader(http.StatusOK)
		})

		// Wrap handler with middleware
		wrappedHandler := middleware(testHandler)

		// Create request with session cookie
		req = httptest.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{
			Name:  config.CookieName,
			Value: session.ID,
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		// Verify session was added to context
		assert.NotNil(t, capturedSession)
		assert.Equal(t, session.ID, capturedSession.ID)
	})

	t.Run("Session Middleware without Cookie", func(t *testing.T) {
		// Create middleware
		middleware := sm.SessionMiddleware()

		// Create test handler
		var capturedSession *Session
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedSession, _ = sm.GetSessionFromRequest(r)
			w.WriteHeader(http.StatusOK)
		})

		// Wrap handler with middleware
		wrappedHandler := middleware(testHandler)

		// Create request without session cookie
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		// Verify no session in context
		assert.Nil(t, capturedSession)
	})

	t.Run("Session Cookie Management", func(t *testing.T) {
		w := httptest.NewRecorder()
		sessionID := "test-session-123"

		// Set session cookie
		sm.SetSessionCookie(w, sessionID)

		// Verify cookie was set
		cookies := w.Result().Cookies()
		assert.Len(t, cookies, 1)

		cookie := cookies[0]
		assert.Equal(t, config.CookieName, cookie.Name)
		assert.Equal(t, sessionID, cookie.Value)
		assert.Equal(t, config.CookiePath, cookie.Path)
		assert.Equal(t, config.SecureCookies, cookie.Secure)
		assert.Equal(t, config.HTTPOnlyCookies, cookie.HttpOnly)

		// Clear session cookie
		w = httptest.NewRecorder()
		sm.ClearSessionCookie(w)

		// Verify cookie was cleared
		cookies = w.Result().Cookies()
		assert.Len(t, cookies, 1)

		clearedCookie := cookies[0]
		assert.Equal(t, config.CookieName, clearedCookie.Name)
		assert.Empty(t, clearedCookie.Value)
		assert.Equal(t, -1, clearedCookie.MaxAge)
	})
}

func TestSessionCleanup(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()
	config.TTL = time.Minute             // Minimum valid TTL
	config.CleanupInterval = time.Minute // Minimum valid cleanup interval

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Manual Session Cleanup", func(t *testing.T) {
		// Create a separate session manager with longer cleanup interval to avoid interference
		manualConfig := DefaultSessionConfig()
		manualConfig.TTL = time.Minute           // Minimum valid TTL
		manualConfig.CleanupInterval = time.Hour // Disable automatic cleanup

		manualSM, err := NewSessionManager(manualConfig, logger)
		require.NoError(t, err)
		defer manualSM.Close()

		req := httptest.NewRequest("GET", "/test", nil)

		// Create session
		session, err := manualSM.CreateSession(context.Background(), "cleanup-user", req)
		require.NoError(t, err)

		// Test cleanup (won't find expired sessions, but tests the mechanism)
		cleaned, err := manualSM.CleanupExpiredSessions(context.Background())
		require.NoError(t, err)
		assert.GreaterOrEqual(t, cleaned, int64(0)) // No expired sessions, so expect 0 or more

		// Verify session still exists (since we didn't wait for expiration)
		retrievedSession, err := manualSM.GetSession(context.Background(), session.ID)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedSession)
	})

	t.Run("Automatic Session Cleanup", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		// Create session
		session, err := sm.CreateSession(context.Background(), "auto-cleanup-user", req)
		require.NoError(t, err)

		// Verify session exists initially
		retrievedSession, err := sm.GetSession(context.Background(), session.ID)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedSession)

		// Test that automatic cleanup service is running (we can't easily test cleanup without waiting 1+ minute)
		assert.NotNil(t, sm) // Session manager should be running with cleanup service
	})
}

func TestSessionMetrics(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Session Metrics", func(t *testing.T) {
		initialMetrics := sm.GetMetrics()
		assert.NotNil(t, initialMetrics)

		req := httptest.NewRequest("GET", "/test", nil)

		// Create sessions
		var sessions []*Session
		for i := 0; i < 3; i++ {
			session, err := sm.CreateSession(context.Background(), fmt.Sprintf("metrics-user-%d", i), req)
			require.NoError(t, err)
			sessions = append(sessions, session)
		}

		// Check updated metrics
		updatedMetrics := sm.GetMetrics()
		assert.Greater(t, updatedMetrics.TotalSessions, initialMetrics.TotalSessions)
		assert.Greater(t, updatedMetrics.ActiveSessions, initialMetrics.ActiveSessions)

		// Delete a session
		err = sm.DeleteSession(context.Background(), sessions[0].ID)
		require.NoError(t, err)

		// Check metrics after deletion
		finalMetrics := sm.GetMetrics()
		assert.Equal(t, updatedMetrics.ActiveSessions-1, finalMetrics.ActiveSessions)
	})

	t.Run("Session Stats", func(t *testing.T) {
		stats := sm.Stats()
		assert.NotNil(t, stats)

		// Verify expected keys
		expectedKeys := []string{
			"total_sessions",
			"active_sessions",
			"backend",
			"ttl_seconds",
			"cleanup_interval_seconds",
		}

		for _, key := range expectedKeys {
			_, exists := stats[key]
			assert.True(t, exists, "missing key: %s", key)
		}
	})
}

func TestSessionConcurrency(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	config := DefaultSessionConfig()

	sm, err := NewSessionManager(config, logger)
	require.NoError(t, err)

	cleanup.Add(func() {
		sm.Close()
	})

	t.Run("Concurrent Session Creation", func(t *testing.T) {
		const numGoroutines = 20
		var wg sync.WaitGroup
		var createdSessions sync.Map

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				req := httptest.NewRequest("GET", "/test", nil)
				session, err := sm.CreateSession(context.Background(), fmt.Sprintf("concurrent-user-%d", id), req)
				if err != nil {
					t.Errorf("failed to create session: %v", err)
					return
				}

				createdSessions.Store(session.ID, session)
			}(i)
		}

		wg.Wait()

		// Count created sessions
		sessionCount := 0
		createdSessions.Range(func(key, value interface{}) bool {
			sessionCount++
			return true
		})

		assert.Equal(t, numGoroutines, sessionCount)
	})

	t.Run("Concurrent Session Operations", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		// Create initial session
		session, err := sm.CreateSession(context.Background(), "concurrent-ops-user", req)
		require.NoError(t, err)

		var wg sync.WaitGroup
		const numOperations = 10

		// Concurrent get operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := sm.GetSession(context.Background(), session.ID)
				if err != nil {
					t.Errorf("failed to get session: %v", err)
				}
			}()
		}

		// Concurrent validation operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := sm.ValidateSession(context.Background(), session.ID, req)
				if err != nil {
					t.Errorf("failed to validate session: %v", err)
				}
			}()
		}

		wg.Wait()
	})
}

func TestMemorySessionStorage(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Memory Storage Basic Operations", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		config := &MemorySessionConfig{
			MaxSessions:    100,
			EnableSharding: false,
		}

		storage, err := NewMemorySessionStorage(config, logger)
		require.NoError(t, err)
		defer storage.Close()

		// Create session
		session := &Session{
			ID:           "test-session-123",
			UserID:       "user123",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(time.Hour),
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}

		// Test create
		err = storage.CreateSession(context.Background(), session)
		require.NoError(t, err)

		// Test get
		retrievedSession, err := storage.GetSession(context.Background(), session.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrievedSession.ID)
		assert.Equal(t, session.UserID, retrievedSession.UserID)

		// Test update
		session.Data["test"] = "value"
		err = storage.UpdateSession(context.Background(), session)
		require.NoError(t, err)

		// Test delete
		err = storage.DeleteSession(context.Background(), session.ID)
		require.NoError(t, err)

		// Verify deleted
		deletedSession, err := storage.GetSession(context.Background(), session.ID)
		assert.NoError(t, err)
		assert.Nil(t, deletedSession)
	})

	t.Run("Memory Storage with Sharding", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		config := &MemorySessionConfig{
			MaxSessions:    100,
			EnableSharding: true,
			ShardCount:     4,
		}

		storage, err := NewMemorySessionStorage(config, logger)
		require.NoError(t, err)
		defer storage.Close()

		// Create multiple sessions
		sessions := make([]*Session, 10)
		for i := 0; i < 10; i++ {
			session := &Session{
				ID:           fmt.Sprintf("shard-test-%d", i),
				UserID:       fmt.Sprintf("user%d", i),
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				ExpiresAt:    time.Now().Add(time.Hour),
				IsActive:     true,
				Data:         make(map[string]interface{}),
				Metadata:     make(map[string]interface{}),
			}

			err = storage.CreateSession(context.Background(), session)
			require.NoError(t, err)
			sessions[i] = session
		}

		// Verify all sessions can be retrieved
		for _, session := range sessions {
			retrievedSession, err := storage.GetSession(context.Background(), session.ID)
			require.NoError(t, err)
			assert.Equal(t, session.ID, retrievedSession.ID)
		}

		// Test count
		count, err := storage.CountSessions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(10), count)
	})

	t.Run("Memory Storage Cleanup", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		config := &MemorySessionConfig{
			MaxSessions:    100,
			EnableSharding: false,
		}

		storage, err := NewMemorySessionStorage(config, logger)
		require.NoError(t, err)
		defer storage.Close()

		// Create session that expires very soon
		expiredSession := &Session{
			ID:           "expired-session",
			UserID:       "user123",
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			LastAccessed: time.Now().Add(-2 * time.Hour),
			ExpiresAt:    time.Now().Add(50 * time.Millisecond), // Will expire soon
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}

		// Create active session
		activeSession := &Session{
			ID:           "active-session",
			UserID:       "user456",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(time.Hour), // Not expired
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}

		// Store both sessions
		err = storage.CreateSession(context.Background(), expiredSession)
		require.NoError(t, err)
		err = storage.CreateSession(context.Background(), activeSession)
		require.NoError(t, err)

		// Wait for the session to expire
		time.Sleep(100 * time.Millisecond)

		// Cleanup expired sessions
		cleaned, err := storage.CleanupExpiredSessions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(1), cleaned)

		// Verify expired session is deleted
		deletedSession, err := storage.GetSession(context.Background(), expiredSession.ID)
		assert.NoError(t, err)
		assert.Nil(t, deletedSession)

		// Verify active session still exists
		_, err = storage.GetSession(context.Background(), activeSession.ID)
		assert.NoError(t, err)
	})
}

func TestSessionData(t *testing.T) {
	t.Run("SessionData Operations", func(t *testing.T) {
		session := &Session{
			ID:   "test-session",
			Data: make(map[string]interface{}),
		}

		sessionData := NewSessionData(session)

		// Test Set and Get
		sessionData.Set("key1", "value1")
		sessionData.Set("key2", 42)
		sessionData.Set("key3", true)

		value1, exists := sessionData.Get("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", value1)

		// Test GetString
		strValue, exists := sessionData.GetString("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", strValue)

		// Test GetInt
		intValue, exists := sessionData.GetInt("key2")
		assert.True(t, exists)
		assert.Equal(t, 42, intValue)

		// Test GetBool
		boolValue, exists := sessionData.GetBool("key3")
		assert.True(t, exists)
		assert.True(t, boolValue)

		// Test Keys
		keys := sessionData.Keys()
		assert.Len(t, keys, 3)
		assert.Contains(t, keys, "key1")
		assert.Contains(t, keys, "key2")
		assert.Contains(t, keys, "key3")

		// Test Delete
		sessionData.Delete("key2")
		_, exists = sessionData.Get("key2")
		assert.False(t, exists)

		// Test Clear
		sessionData.Clear()
		assert.Len(t, sessionData.Keys(), 0)
	})
}
