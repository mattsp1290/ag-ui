package state

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/common"
	"go.uber.org/zap"
)

// validateWebhookURL validates a webhook URL to prevent SSRF attacks
// This function now uses the consolidated URL validator from common package
// but retains backward compatibility and enhanced validation
func validateWebhookURL(urlStr string) error {
	// Use the consolidated URL validator with webhook-specific options
	opts := common.DefaultWebhookValidationOptions()
	return common.ValidateURL(urlStr, opts)
}

// isInternalIP checks if an IP address is in internal/private ranges
// This now delegates to the common implementation to ensure consistency
func isInternalIP(ip net.IP) bool {
	return common.IsInternalIP(ip)
}

// LogAlertNotifier sends alerts to a logger
type LogAlertNotifier struct {
	logger *zap.Logger
}

// NewLogAlertNotifier creates a new log alert notifier
func NewLogAlertNotifier(logger *zap.Logger) *LogAlertNotifier {
	return &LogAlertNotifier{
		logger: logger,
	}
}

// SendAlert sends an alert to the logger
func (n *LogAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	switch alert.Level {
	case AlertLevelInfo:
		n.logger.Info("Alert",
			zap.String("title", alert.Title),
			zap.String("description", alert.Description),
			zap.String("component", alert.Component),
			zap.Float64("value", alert.Value),
			zap.Float64("threshold", alert.Threshold),
			zap.Any("labels", alert.Labels))
	case AlertLevelWarning:
		n.logger.Warn("Alert",
			zap.String("title", alert.Title),
			zap.String("description", alert.Description),
			zap.String("component", alert.Component),
			zap.Float64("value", alert.Value),
			zap.Float64("threshold", alert.Threshold),
			zap.Any("labels", alert.Labels))
	case AlertLevelError:
		n.logger.Error("Alert",
			zap.String("title", alert.Title),
			zap.String("description", alert.Description),
			zap.String("component", alert.Component),
			zap.Float64("value", alert.Value),
			zap.Float64("threshold", alert.Threshold),
			zap.Any("labels", alert.Labels))
	case AlertLevelCritical:
		n.logger.Error("Critical Alert",
			zap.String("title", alert.Title),
			zap.String("description", alert.Description),
			zap.String("component", alert.Component),
			zap.Float64("value", alert.Value),
			zap.Float64("threshold", alert.Threshold),
			zap.Any("labels", alert.Labels))
	}

	return nil
}

// EmailAlertNotifier sends alerts via email (placeholder implementation)
type EmailAlertNotifier struct {
	smtpServer string
	smtpPort   int
	username   string
	password   string
	from       string
	to         []string
	subject    string
	enabled    bool
}

// NewEmailAlertNotifier creates a new email alert notifier
func NewEmailAlertNotifier(smtpServer string, smtpPort int, username, password, from string, to []string) *EmailAlertNotifier {
	return &EmailAlertNotifier{
		smtpServer: smtpServer,
		smtpPort:   smtpPort,
		username:   username,
		password:   password,
		from:       from,
		to:         to,
		subject:    "State Manager Alert",
		enabled:    true,
	}
}

// SendAlert sends an alert via email
func (n *EmailAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	if !n.enabled {
		return nil
	}

	// For now, just log that we would send an email
	// In a real implementation, you would use an SMTP library
	fmt.Printf("EMAIL ALERT: %s - %s\n", alert.Title, alert.Description)
	return nil
}

// WebhookAlertNotifier sends alerts to a webhook endpoint
type WebhookAlertNotifier struct {
	url     string
	method  string
	headers map[string]string
	timeout time.Duration
	client  *http.Client
}

// NewWebhookAlertNotifier creates a new webhook alert notifier
func NewWebhookAlertNotifier(url string, timeout time.Duration) (*WebhookAlertNotifier, error) {
	// Validate the webhook URL to prevent SSRF attacks
	if err := validateWebhookURL(url); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Create HTTP client with secure TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
		// Prevent connection reuse that could bypass URL validation
		DisableKeepAlives: true,
	}

	return &WebhookAlertNotifier{
		url:     url,
		method:  "POST",
		headers: make(map[string]string),
		timeout: timeout,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}, nil
}

// NewWebhookAlertNotifierForTesting creates a webhook notifier without URL validation (for testing only)
func NewWebhookAlertNotifierForTesting(url string, timeout time.Duration) (*WebhookAlertNotifier, error) {
	// Create HTTP client with secure TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			InsecureSkipVerify: true, // For testing with self-signed certs
		},
		// Prevent connection reuse that could bypass URL validation
		DisableKeepAlives: true,
	}

	return &WebhookAlertNotifier{
		url:     url,
		method:  "POST",
		headers: make(map[string]string),
		timeout: timeout,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}, nil
}

// SetHeader sets a header for the webhook request
func (n *WebhookAlertNotifier) SetHeader(key, value string) {
	n.headers[key] = value
}

// SendAlert sends an alert to a webhook endpoint
func (n *WebhookAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	payload := map[string]interface{}{
		"timestamp":   alert.Timestamp.Format(time.RFC3339),
		"level":       alertLevelToString(alert.Level),
		"title":       alert.Title,
		"description": alert.Description,
		"component":   alert.Component,
		"value":       alert.Value,
		"threshold":   alert.Threshold,
		"labels":      alert.Labels,
		"severity":    auditSeverityToString(alert.Severity),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal alert payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, n.method, n.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range n.headers {
		req.Header.Set(key, value)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SlackAlertNotifier sends alerts to Slack
type SlackAlertNotifier struct {
	webhookURL string
	channel    string
	username   string
	client     *http.Client
}

// NewSlackAlertNotifier creates a new Slack alert notifier
func NewSlackAlertNotifier(webhookURL, channel, username string) (*SlackAlertNotifier, error) {
	// Validate the webhook URL
	if err := validateWebhookURL(webhookURL); err != nil {
		return nil, fmt.Errorf("invalid Slack webhook URL: %w", err)
	}

	return &SlackAlertNotifier{
		webhookURL: webhookURL,
		channel:    channel,
		username:   username,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// SendAlert sends an alert to Slack
func (n *SlackAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	color := n.getColorForLevel(alert.Level)

	payload := map[string]interface{}{
		"channel":  n.channel,
		"username": n.username,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": alert.Title,
				"text":  alert.Description,
				"fields": []map[string]interface{}{
					{
						"title": "Component",
						"value": alert.Component,
						"short": true,
					},
					{
						"title": "Value",
						"value": fmt.Sprintf("%.2f", alert.Value),
						"short": true,
					},
					{
						"title": "Threshold",
						"value": fmt.Sprintf("%.2f", alert.Threshold),
						"short": true,
					},
					{
						"title": "Timestamp",
						"value": alert.Timestamp.Format(time.RFC3339),
						"short": true,
					},
				},
				"footer": "State Manager",
				"ts":     alert.Timestamp.Unix(),
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slack request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (n *SlackAlertNotifier) getColorForLevel(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "good"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelError:
		return "danger"
	case AlertLevelCritical:
		return "danger"
	default:
		return "good"
	}
}

// PagerDutyAlertNotifier sends alerts to PagerDuty
type PagerDutyAlertNotifier struct {
	integrationKey string
	client         *http.Client
}

// NewPagerDutyAlertNotifier creates a new PagerDuty alert notifier
func NewPagerDutyAlertNotifier(integrationKey string) *PagerDutyAlertNotifier {
	return &PagerDutyAlertNotifier{
		integrationKey: integrationKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendAlert sends an alert to PagerDuty
func (n *PagerDutyAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	eventAction := "trigger"
	if alert.Level == AlertLevelInfo {
		eventAction = "resolve"
	}

	payload := map[string]interface{}{
		"routing_key":  n.integrationKey,
		"event_action": eventAction,
		"dedup_key":    fmt.Sprintf("state-manager-%s-%s", alert.Component, alert.Title),
		"payload": map[string]interface{}{
			"summary":   alert.Title,
			"source":    "state-manager",
			"severity":  n.getSeverityForLevel(alert.Level),
			"component": alert.Component,
			"custom_details": map[string]interface{}{
				"description": alert.Description,
				"value":       alert.Value,
				"threshold":   alert.Threshold,
				"labels":      alert.Labels,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://events.pagerduty.com/v2/enqueue", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PagerDuty request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PagerDuty request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (n *PagerDutyAlertNotifier) getSeverityForLevel(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "info"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelError:
		return "error"
	case AlertLevelCritical:
		return "critical"
	default:
		return "info"
	}
}

// FileAlertNotifier writes alerts to a file
type FileAlertNotifier struct {
	filename string
	file     *os.File
}

// NewFileAlertNotifier creates a new file alert notifier
func NewFileAlertNotifier(filename string) (*FileAlertNotifier, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open alert file: %w", err)
	}

	return &FileAlertNotifier{
		filename: filename,
		file:     file,
	}, nil
}

// SendAlert writes an alert to a file
func (n *FileAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	alertData := map[string]interface{}{
		"timestamp":   alert.Timestamp.Format(time.RFC3339),
		"level":       alertLevelToString(alert.Level),
		"title":       alert.Title,
		"description": alert.Description,
		"component":   alert.Component,
		"value":       alert.Value,
		"threshold":   alert.Threshold,
		"labels":      alert.Labels,
		"severity":    auditSeverityToString(alert.Severity),
	}

	jsonData, err := json.Marshal(alertData)
	if err != nil {
		return fmt.Errorf("failed to marshal alert data: %w", err)
	}

	_, err = n.file.Write(append(jsonData, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write alert to file: %w", err)
	}

	return n.file.Sync()
}

// Close closes the file
func (n *FileAlertNotifier) Close() error {
	if n.file != nil {
		return n.file.Close()
	}
	return nil
}

// CompositeAlertNotifier sends alerts to multiple notifiers
type CompositeAlertNotifier struct {
	notifiers []AlertNotifier
}

// NewCompositeAlertNotifier creates a new composite alert notifier
func NewCompositeAlertNotifier(notifiers ...AlertNotifier) *CompositeAlertNotifier {
	return &CompositeAlertNotifier{
		notifiers: notifiers,
	}
}

// SendAlert sends an alert to all configured notifiers
func (n *CompositeAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	var errors []string

	for _, notifier := range n.notifiers {
		if err := notifier.SendAlert(ctx, alert); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send alert to some notifiers: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ConditionalAlertNotifier sends alerts based on conditions
type ConditionalAlertNotifier struct {
	notifier  AlertNotifier
	condition func(Alert) bool
}

// NewConditionalAlertNotifier creates a new conditional alert notifier
func NewConditionalAlertNotifier(notifier AlertNotifier, condition func(Alert) bool) *ConditionalAlertNotifier {
	return &ConditionalAlertNotifier{
		notifier:  notifier,
		condition: condition,
	}
}

// SendAlert sends an alert if the condition is met
func (n *ConditionalAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	if n.condition(alert) {
		return n.notifier.SendAlert(ctx, alert)
	}
	return nil
}

// ThrottledAlertNotifier prevents alert spam by throttling
type ThrottledAlertNotifier struct {
	notifier         AlertNotifier
	lastSent         map[string]time.Time
	throttleDuration time.Duration
	mu               sync.RWMutex // Protects lastSent map from concurrent access
}

// NewThrottledAlertNotifier creates a new throttled alert notifier
func NewThrottledAlertNotifier(notifier AlertNotifier, throttleDuration time.Duration) *ThrottledAlertNotifier {
	return &ThrottledAlertNotifier{
		notifier:         notifier,
		lastSent:         make(map[string]time.Time),
		throttleDuration: throttleDuration,
	}
}

// SendAlert sends an alert if it hasn't been sent recently
func (n *ThrottledAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	alertKey := fmt.Sprintf("%s_%s", alert.Component, alert.Title)

	// Check if alert was sent recently (read operation)
	n.mu.RLock()
	lastSent, exists := n.lastSent[alertKey]
	n.mu.RUnlock()

	if exists && time.Since(lastSent) < n.throttleDuration {
		return nil // Skip sending, too recent
	}

	err := n.notifier.SendAlert(ctx, alert)
	if err == nil {
		// Update the last sent time (write operation)
		n.mu.Lock()
		n.lastSent[alertKey] = time.Now()
		n.mu.Unlock()
	}

	return err
}

// Helper functions

func alertLevelToString(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "info"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelError:
		return "error"
	case AlertLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

func auditSeverityToString(severity AuditSeverityLevel) string {
	switch severity {
	case AuditSeverityDebug:
		return "debug"
	case AuditSeverityInfo:
		return "info"
	case AuditSeverityWarning:
		return "warning"
	case AuditSeverityError:
		return "error"
	case AuditSeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}
