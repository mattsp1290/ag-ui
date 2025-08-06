package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	testutils "github.com/mattsp1290/ag-ui/go-sdk/pkg/testing"
	"go.uber.org/zap/zaptest"
)

func TestEnhancedCertificateManagerLifecycle(t *testing.T) {
	// Create temporary certificates for testing
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "server.crt")
	keyFile := filepath.Join(tempDir, "server.key")
	
	// Generate test certificate and key
	if err := generateTestCertificates(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificates: %v", err)
	}
	
	t.Run("BasicLifecycle", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaks(t, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			// Start monitoring
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager: %v", err)
			}
			
			// Verify it's healthy
			healthy, err := manager.IsHealthy()
			if err != nil {
				t.Fatalf("Health check failed: %v", err)
			}
			if !healthy {
				t.Error("Certificate manager should be healthy")
			}
			
			// Get TLS config
			tlsConfig, err := manager.GetTLSConfig()
			if err != nil {
				t.Fatalf("Failed to get TLS config: %v", err)
			}
			if len(tlsConfig.Certificates) == 0 {
				t.Error("TLS config should have certificates")
			}
			
			// Stop gracefully
			if err := manager.Stop(5 * time.Second); err != nil {
				t.Fatalf("Failed to stop certificate manager: %v", err)
			}
			
			// Should not be healthy after stop
			healthy, _ = manager.IsHealthy()
			if healthy {
				t.Error("Certificate manager should not be healthy after stop")
			}
		})
	})
	
	t.Run("CertificateReloading", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaksWithOptions(t, func(detector *testutils.EnhancedGoroutineLeakDetector) {
			detector.WithTolerance(2).WithMaxWaitTime(10 * time.Second)
		}, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			// Set a short watch interval for testing
			manager.SetWatchInterval(100 * time.Millisecond)
			
			// Track reload events
			var reloadCount int64
			manager.AddReloadCallback(func(*tls.Config) {
				atomic.AddInt64(&reloadCount, 1)
			})
			
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager: %v", err)
			}
			defer func() {
				if err := manager.Stop(5 * time.Second); err != nil {
					t.Errorf("Failed to stop certificate manager: %v", err)
				}
			}()
			
			// Wait a bit then touch the certificate file
			time.Sleep(200 * time.Millisecond)
			
			// Modify the certificate file to trigger reload
			file, err := os.OpenFile(certFile, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				t.Fatalf("Failed to open certificate file: %v", err)
			}
			file.WriteString("\n# Modified for test\n")
			file.Close()
			
			// Wait for reload to be detected
			maxWait := time.Now().Add(2 * time.Second)
			for atomic.LoadInt64(&reloadCount) == 0 && time.Now().Before(maxWait) {
				time.Sleep(50 * time.Millisecond)
			}
			
			if atomic.LoadInt64(&reloadCount) == 0 {
				t.Error("Certificate reload was not detected")
			}
		})
	})
	
	t.Run("GracefulShutdown", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaks(t, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager: %v", err)
			}
			
			// Shutdown should complete quickly
			start := time.Now()
			if err := manager.Stop(5 * time.Second); err != nil {
				t.Fatalf("Failed to stop certificate manager: %v", err)
			}
			elapsed := time.Since(start)
			
			if elapsed > 1*time.Second {
				t.Errorf("Shutdown took too long: %v", elapsed)
			}
		})
	})
	
	t.Run("ShutdownTimeout", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaksWithOptions(t, func(detector *testutils.EnhancedGoroutineLeakDetector) {
			detector.WithTolerance(1).WithMaxWaitTime(3 * time.Second)
		}, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			// Set a short watch interval to have active monitoring
			manager.SetWatchInterval(10 * time.Millisecond)
			
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager: %v", err)
			}
			
			// Wait a bit to ensure monitoring is active
			time.Sleep(50 * time.Millisecond)
			
			// Use a timeout that should be too short for graceful shutdown
			// when there's active monitoring
			start := time.Now()
			err = manager.Stop(1 * time.Nanosecond) // Extremely short timeout
			elapsed := time.Since(start)
			
			// Either we get a timeout error, or shutdown completes extremely quickly
			if err == nil && elapsed > 10*time.Millisecond {
				t.Error("Expected shutdown timeout error for extremely short timeout")
			}
			
			// Proper cleanup - give more time
			_ = manager.Stop(5 * time.Second)
		})
	})
	
	t.Run("MultipleStartStop", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaks(t, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			// First start should succeed
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager on first attempt: %v", err)
			}
			
			// Multiple starts should fail while already started
			for i := 0; i < 2; i++ {
				if err := manager.Start(); err == nil {
					t.Error("Manager should not allow multiple starts")
				}
			}
			
			// Stop should succeed
			if err := manager.Stop(2 * time.Second); err != nil {
				t.Fatalf("Failed to stop certificate manager: %v", err)
			}
			
			// Restart should succeed after stop
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to restart certificate manager after stop: %v", err)
			}
			
			// Final stop
			if err := manager.Stop(2 * time.Second); err != nil {
				t.Fatalf("Failed to stop certificate manager: %v", err)
			}
		})
	})
	
	t.Run("ConcurrentOperations", func(t *testing.T) {
		testutils.VerifyNoGoroutineLeaksWithOptions(t, func(detector *testutils.EnhancedGoroutineLeakDetector) {
			detector.WithTolerance(3).WithMaxWaitTime(5 * time.Second)
		}, func() {
			logger := zaptest.NewLogger(t)
			config := &TLSConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			}
			
			manager, err := NewEnhancedCertificateManager(config, logger)
			if err != nil {
				t.Fatalf("Failed to create certificate manager: %v", err)
			}
			
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start certificate manager: %v", err)
			}
			defer func() {
				if err := manager.Stop(5 * time.Second); err != nil {
					t.Errorf("Failed to stop certificate manager: %v", err)
				}
			}()
			
			// Perform concurrent operations
			done := make(chan struct{})
			defer close(done)
			
			// Concurrent TLS config access
			for i := 0; i < 5; i++ {
				go func() {
					for {
						select {
						case <-done:
							return
						default:
							_, _ = manager.GetTLSConfig()
							_, _ = manager.GetCertificateInfo()
							_, _ = manager.IsHealthy()
							time.Sleep(10 * time.Millisecond)
						}
					}
				}()
			}
			
			// Let operations run
			time.Sleep(200 * time.Millisecond)
		})
	})
}

// generateTestCertificates creates a self-signed certificate for testing
func generateTestCertificates(certFile, keyFile string) error {
	// Generate a private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	
	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Org"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
	}
	
	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	
	// Encode certificate to PEM
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()
	
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}
	
	// Encode private key to PEM
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}); err != nil {
		return err
	}
	
	return nil
}