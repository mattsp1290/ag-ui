package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CertificateManager handles TLS certificate management and validation
type CertificateManager struct {
	config          *TLSConfig
	logger          *zap.Logger
	tlsConfig       *tls.Config
	certificates    []tls.Certificate
	caCertPool      *x509.CertPool
	mu              sync.RWMutex
	certWatcher     *CertificateWatcher
	lastReload      time.Time
}

// CertificateWatcher monitors certificate files for changes
type CertificateWatcher struct {
	certFile     string
	keyFile      string
	caFile       string
	lastModTime  map[string]time.Time
	stopChan     chan bool
	reloadChan   chan bool
	manager      *CertificateManager
}

// CertificateInfo contains information about a certificate
type CertificateInfo struct {
	Subject       string    `json:"subject"`
	Issuer        string    `json:"issuer"`
	SerialNumber  string    `json:"serial_number"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	DNSNames      []string  `json:"dns_names"`
	IPAddresses   []string  `json:"ip_addresses"`
	KeyUsage      []string  `json:"key_usage"`
	IsCA          bool      `json:"is_ca"`
	IsExpired     bool      `json:"is_expired"`
	DaysUntilExpiry int     `json:"days_until_expiry"`
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(config *TLSConfig, logger *zap.Logger) (*CertificateManager, error) {
	if config == nil {
		return nil, fmt.Errorf("TLS config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	cm := &CertificateManager{
		config:     config,
		logger:     logger,
		lastReload: time.Now(),
	}
	
	// Load certificates
	if err := cm.loadCertificates(); err != nil {
		return nil, fmt.Errorf("failed to load certificates: %w", err)
	}
	
	// Create TLS configuration
	if err := cm.createTLSConfig(); err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}
	
	// Start certificate watcher if files are configured
	if config.CertFile != "" && config.KeyFile != "" {
		cm.startCertificateWatcher()
	}
	
	return cm, nil
}

// loadCertificates loads certificates from files
func (cm *CertificateManager) loadCertificates() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Load server certificate and key
	if cm.config.CertFile != "" && cm.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load certificate and key: %w", err)
		}
		
		cm.certificates = []tls.Certificate{cert}
		
		cm.logger.Info("Loaded server certificate",
			zap.String("cert_file", cm.config.CertFile),
			zap.String("key_file", cm.config.KeyFile))
	}
	
	// Load CA certificates
	if cm.config.CAFile != "" {
		caCertPEM, err := os.ReadFile(cm.config.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		
		cm.caCertPool = x509.NewCertPool()
		if !cm.caCertPool.AppendCertsFromPEM(caCertPEM) {
			return fmt.Errorf("failed to parse CA certificate")
		}
		
		cm.logger.Info("Loaded CA certificates",
			zap.String("ca_file", cm.config.CAFile))
	}
	
	return nil
}

// createTLSConfig creates the TLS configuration
func (cm *CertificateManager) createTLSConfig() error {
	tlsConfig := &tls.Config{
		MinVersion:   cm.config.MinVersion,
		MaxVersion:   cm.config.MaxVersion,
		ClientAuth:   cm.config.ClientAuth,
		Certificates: cm.certificates,
	}
	
	// Set cipher suites if configured
	if len(cm.config.CipherSuites) > 0 {
		tlsConfig.CipherSuites = cm.config.CipherSuites
	}
	
	// Set curve preferences if configured
	if len(cm.config.CurvePreferences) > 0 {
		tlsConfig.CurvePreferences = cm.config.CurvePreferences
	}
	
	// Set CA pool for client certificate verification
	if cm.caCertPool != nil {
		tlsConfig.ClientCAs = cm.caCertPool
		tlsConfig.RootCAs = cm.caCertPool
	}
	
	// Configure certificate validation
	if cm.config.CertificateValidation.ValidateCertChain ||
	   cm.config.CertificateValidation.ValidateHostname {
		tlsConfig.VerifyPeerCertificate = cm.verifyPeerCertificate
	}
	
	// Set InsecureSkipVerify if configured
	tlsConfig.InsecureSkipVerify = cm.config.InsecureSkipVerify
	
	// Set server name indication
	if !cm.config.EnableSNI {
		tlsConfig.ServerName = ""
	}
	
	cm.mu.Lock()
	cm.tlsConfig = tlsConfig
	cm.mu.Unlock()
	
	cm.logger.Info("Created TLS configuration",
		zap.Uint16("min_version", cm.config.MinVersion),
		zap.Uint16("max_version", cm.config.MaxVersion),
		zap.String("client_auth", clientAuthTypeToString(cm.config.ClientAuth)),
		zap.Int("cipher_suites", len(cm.config.CipherSuites)),
		zap.Bool("insecure_skip_verify", cm.config.InsecureSkipVerify))
	
	return nil
}

// verifyPeerCertificate performs custom certificate verification
func (cm *CertificateManager) verifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no certificates provided")
	}
	
	// Parse the first certificate
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	
	// Validate certificate chain
	if cm.config.CertificateValidation.ValidateCertChain {
		if err := cm.validateCertificateChain(cert, verifiedChains); err != nil {
			return fmt.Errorf("certificate chain validation failed: %w", err)
		}
	}
	
	// Validate hostname
	if cm.config.CertificateValidation.ValidateHostname {
		if err := cm.validateHostname(cert); err != nil {
			return fmt.Errorf("hostname validation failed: %w", err)
		}
	}
	
	// Check certificate revocation (CRL)
	if cm.config.CertificateValidation.CRLCheckEnabled {
		if err := cm.checkCertificateRevocation(cert); err != nil {
			return fmt.Errorf("certificate revocation check failed: %w", err)
		}
	}
	
	// Check OCSP
	if cm.config.CertificateValidation.OCSPCheckEnabled {
		if err := cm.checkOCSP(cert); err != nil {
			return fmt.Errorf("OCSP check failed: %w", err)
		}
	}
	
	cm.logger.Debug("Peer certificate verified successfully",
		zap.String("subject", cert.Subject.String()),
		zap.String("issuer", cert.Issuer.String()),
		zap.Time("not_after", cert.NotAfter))
	
	return nil
}

// validateCertificateChain validates the certificate chain
func (cm *CertificateManager) validateCertificateChain(cert *x509.Certificate, verifiedChains [][]*x509.Certificate) error {
	// Check if we have verified chains
	if len(verifiedChains) == 0 {
		return fmt.Errorf("no verified certificate chains")
	}
	
	// Check certificate expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid")
	}
	
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}
	
	// Additional chain validation can be added here
	return nil
}

// validateHostname validates the certificate hostname
func (cm *CertificateManager) validateHostname(cert *x509.Certificate) error {
	// In a real implementation, you would check the certificate's DNS names
	// against the expected hostname
	if len(cert.DNSNames) == 0 {
		return fmt.Errorf("certificate has no DNS names")
	}
	
	// Additional hostname validation logic would go here
	return nil
}

// checkCertificateRevocation checks certificate revocation status
func (cm *CertificateManager) checkCertificateRevocation(cert *x509.Certificate) error {
	// Placeholder for CRL checking
	// In a real implementation, this would:
	// 1. Download the CRL from the certificate's CRL distribution points
	// 2. Parse the CRL
	// 3. Check if the certificate serial number is in the revoked list
	
	cm.logger.Debug("CRL check placeholder - certificate assumed valid",
		zap.String("serial", cert.SerialNumber.String()))
	
	return nil
}

// checkOCSP checks certificate status using OCSP
func (cm *CertificateManager) checkOCSP(cert *x509.Certificate) error {
	// Placeholder for OCSP checking
	// In a real implementation, this would:
	// 1. Extract OCSP responder URLs from the certificate
	// 2. Send OCSP requests to the responders
	// 3. Validate the OCSP response
	
	cm.logger.Debug("OCSP check placeholder - certificate assumed valid",
		zap.String("serial", cert.SerialNumber.String()))
	
	return nil
}

// startCertificateWatcher starts monitoring certificate files for changes
func (cm *CertificateManager) startCertificateWatcher() {
	cm.certWatcher = &CertificateWatcher{
		certFile:    cm.config.CertFile,
		keyFile:     cm.config.KeyFile,
		caFile:      cm.config.CAFile,
		lastModTime: make(map[string]time.Time),
		stopChan:    make(chan bool),
		reloadChan:  make(chan bool, 1),
		manager:     cm,
	}
	
	// Record initial modification times
	files := []string{cm.config.CertFile, cm.config.KeyFile}
	if cm.config.CAFile != "" {
		files = append(files, cm.config.CAFile)
	}
	
	for _, file := range files {
		if stat, err := os.Stat(file); err == nil {
			cm.certWatcher.lastModTime[file] = stat.ModTime()
		}
	}
	
	// Start watching goroutine
	go cm.certWatcher.watch()
	
	// Start reload handler goroutine
	go cm.handleCertificateReloads()
	
	cm.logger.Info("Started certificate file watcher",
		zap.String("cert_file", cm.config.CertFile),
		zap.String("key_file", cm.config.KeyFile),
		zap.String("ca_file", cm.config.CAFile))
}

// watch monitors certificate files for changes
func (cw *CertificateWatcher) watch() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-cw.stopChan:
			return
		case <-ticker.C:
			cw.checkForChanges()
		}
	}
}

// checkForChanges checks if any certificate files have changed
func (cw *CertificateWatcher) checkForChanges() {
	files := []string{cw.certFile, cw.keyFile}
	if cw.caFile != "" {
		files = append(files, cw.caFile)
	}
	
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			cw.manager.logger.Warn("Failed to stat certificate file",
				zap.String("file", file),
				zap.Error(err))
			continue
		}
		
		lastMod, exists := cw.lastModTime[file]
		if !exists || stat.ModTime().After(lastMod) {
			cw.manager.logger.Info("Certificate file changed",
				zap.String("file", file),
				zap.Time("old_time", lastMod),
				zap.Time("new_time", stat.ModTime()))
			
			cw.lastModTime[file] = stat.ModTime()
			
			// Trigger reload
			select {
			case cw.reloadChan <- true:
			default:
				// Reload already pending
			}
		}
	}
}

// handleCertificateReloads handles certificate reload events
func (cm *CertificateManager) handleCertificateReloads() {
	for range cm.certWatcher.reloadChan {
		cm.logger.Info("Reloading certificates due to file changes")
		
		if err := cm.ReloadCertificates(); err != nil {
			cm.logger.Error("Failed to reload certificates",
				zap.Error(err))
		} else {
			cm.logger.Info("Successfully reloaded certificates")
		}
	}
}

// GetTLSConfig returns the current TLS configuration
func (cm *CertificateManager) GetTLSConfig() (*tls.Config, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if cm.tlsConfig == nil {
		return nil, fmt.Errorf("TLS config not initialized")
	}
	
	// Return a copy to prevent modification
	return cm.tlsConfig.Clone(), nil
}

// ReloadCertificates reloads certificates from files
func (cm *CertificateManager) ReloadCertificates() error {
	// Load new certificates
	if err := cm.loadCertificates(); err != nil {
		return fmt.Errorf("failed to load certificates: %w", err)
	}
	
	// Recreate TLS configuration
	if err := cm.createTLSConfig(); err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}
	
	cm.lastReload = time.Now()
	
	cm.logger.Info("Reloaded certificates successfully")
	return nil
}

// GetCertificateInfo returns information about the loaded certificates
func (cm *CertificateManager) GetCertificateInfo() ([]*CertificateInfo, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var certInfos []*CertificateInfo
	
	for _, tlsCert := range cm.certificates {
		if len(tlsCert.Certificate) == 0 {
			continue
		}
		
		// Parse the certificate
		cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
		if err != nil {
			cm.logger.Warn("Failed to parse certificate", zap.Error(err))
			continue
		}
		
		// Create certificate info
		certInfo := &CertificateInfo{
			Subject:         cert.Subject.String(),
			Issuer:          cert.Issuer.String(),
			SerialNumber:    cert.SerialNumber.String(),
			NotBefore:       cert.NotBefore,
			NotAfter:        cert.NotAfter,
			DNSNames:        cert.DNSNames,
			IsCA:            cert.IsCA,
		}
		
		// Add IP addresses
		for _, ip := range cert.IPAddresses {
			certInfo.IPAddresses = append(certInfo.IPAddresses, ip.String())
		}
		
		// Add key usage
		certInfo.KeyUsage = keyUsageToStrings(cert.KeyUsage)
		
		// Check if expired
		now := time.Now()
		certInfo.IsExpired = now.After(cert.NotAfter)
		certInfo.DaysUntilExpiry = int(cert.NotAfter.Sub(now).Hours() / 24)
		
		certInfos = append(certInfos, certInfo)
	}
	
	return certInfos, nil
}

// ValidateCertificateChain validates a certificate chain
func (cm *CertificateManager) ValidateCertificateChain(certChain []*x509.Certificate) error {
	if len(certChain) == 0 {
		return fmt.Errorf("empty certificate chain")
	}
	
	// Create verification options
	opts := x509.VerifyOptions{
		Roots: cm.caCertPool,
	}
	
	// Verify the chain
	_, err := certChain[0].Verify(opts)
	if err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}
	
	return nil
}

// CheckCertificateExpiry checks if certificates are close to expiration
func (cm *CertificateManager) CheckCertificateExpiry(warnDays int) []string {
	var warnings []string
	
	certInfos, err := cm.GetCertificateInfo()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to get certificate info: %v", err))
		return warnings
	}
	
	for _, certInfo := range certInfos {
		if certInfo.IsExpired {
			warnings = append(warnings, fmt.Sprintf("Certificate expired: %s (expired %d days ago)",
				certInfo.Subject, -certInfo.DaysUntilExpiry))
		} else if certInfo.DaysUntilExpiry <= warnDays {
			warnings = append(warnings, fmt.Sprintf("Certificate expiring soon: %s (expires in %d days)",
				certInfo.Subject, certInfo.DaysUntilExpiry))
		}
	}
	
	return warnings
}

// GetLastReloadTime returns the last time certificates were reloaded
func (cm *CertificateManager) GetLastReloadTime() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastReload
}

// Cleanup performs cleanup operations
func (cm *CertificateManager) Cleanup() error {
	// Stop certificate watcher
	if cm.certWatcher != nil {
		close(cm.certWatcher.stopChan)
	}
	
	cm.logger.Info("Certificate manager cleanup completed")
	return nil
}

// Helper functions

func clientAuthTypeToString(authType tls.ClientAuthType) string {
	switch authType {
	case tls.NoClientCert:
		return "NoClientCert"
	case tls.RequestClientCert:
		return "RequestClientCert"
	case tls.RequireAnyClientCert:
		return "RequireAnyClientCert"
	case tls.VerifyClientCertIfGiven:
		return "VerifyClientCertIfGiven"
	case tls.RequireAndVerifyClientCert:
		return "RequireAndVerifyClientCert"
	default:
		return "Unknown"
	}
}

func keyUsageToStrings(keyUsage x509.KeyUsage) []string {
	var usages []string
	
	if keyUsage&x509.KeyUsageDigitalSignature != 0 {
		usages = append(usages, "DigitalSignature")
	}
	if keyUsage&x509.KeyUsageContentCommitment != 0 {
		usages = append(usages, "ContentCommitment")
	}
	if keyUsage&x509.KeyUsageKeyEncipherment != 0 {
		usages = append(usages, "KeyEncipherment")
	}
	if keyUsage&x509.KeyUsageDataEncipherment != 0 {
		usages = append(usages, "DataEncipherment")
	}
	if keyUsage&x509.KeyUsageKeyAgreement != 0 {
		usages = append(usages, "KeyAgreement")
	}
	if keyUsage&x509.KeyUsageCertSign != 0 {
		usages = append(usages, "CertSign")
	}
	if keyUsage&x509.KeyUsageCRLSign != 0 {
		usages = append(usages, "CRLSign")
	}
	if keyUsage&x509.KeyUsageEncipherOnly != 0 {
		usages = append(usages, "EncipherOnly")
	}
	if keyUsage&x509.KeyUsageDecipherOnly != 0 {
		usages = append(usages, "DecipherOnly")
	}
	
	return usages
}