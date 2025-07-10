package negotiation

import (
	"fmt"
	"strconv"
	"strings"
)

// AcceptType represents a single media type from an Accept header
type AcceptType struct {
	Type       string            // The media type (e.g., "application/json")
	Quality    float64           // The quality factor (q-value)
	Parameters map[string]string // Additional parameters
}

// ParseAcceptHeader parses an RFC 7231 compliant Accept header
func ParseAcceptHeader(header string) ([]AcceptType, error) {
	if header == "" {
		return []AcceptType{{Type: "*/*", Quality: 1.0}}, nil
	}

	var acceptTypes []AcceptType

	// Split by comma to get individual media types
	parts := strings.Split(header, ",")

	for _, part := range parts {
		acceptType, err := parseAcceptType(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid accept type '%s': %w", part, err)
		}
		acceptTypes = append(acceptTypes, acceptType)
	}

	// Sort by quality factor (highest first)
	sortAcceptTypes(acceptTypes)

	return acceptTypes, nil
}

// parseAcceptType parses a single accept type with parameters
func parseAcceptType(s string) (AcceptType, error) {
	if s == "" {
		return AcceptType{}, fmt.Errorf("empty accept type")
	}

	acceptType := AcceptType{
		Quality:    1.0, // Default quality
		Parameters: make(map[string]string),
	}

	// Split by semicolon to separate media type from parameters
	parts := strings.Split(s, ";")
	
	// First part is the media type
	acceptType.Type = strings.TrimSpace(parts[0])
	if acceptType.Type == "" {
		return AcceptType{}, fmt.Errorf("empty media type")
	}

	// Validate media type format
	if !isValidMediaType(acceptType.Type) {
		return AcceptType{}, fmt.Errorf("invalid media type format: %s", acceptType.Type)
	}

	// Parse parameters
	for i := 1; i < len(parts); i++ {
		param := strings.TrimSpace(parts[i])
		if param == "" {
			continue
		}

		// Split parameter by equals sign
		paramParts := strings.SplitN(param, "=", 2)
		if len(paramParts) != 2 {
			return AcceptType{}, fmt.Errorf("invalid parameter format: %s", param)
		}

		key := strings.TrimSpace(paramParts[0])
		value := strings.TrimSpace(paramParts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"")

		// Handle q-value specially
		if key == "q" {
			q, err := parseQuality(value)
			if err != nil {
				return AcceptType{}, fmt.Errorf("invalid q-value: %w", err)
			}
			acceptType.Quality = q
		} else {
			acceptType.Parameters[key] = value
		}
	}

	return acceptType, nil
}

// parseQuality parses a quality factor (q-value)
func parseQuality(s string) (float64, error) {
	// RFC 7231: qvalue = ( "0" [ "." 0*3DIGIT ] ) / ( "1" [ "." 0*3("0") ] )
	q, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	// Validate range
	if q < 0 || q > 1 {
		return 0, fmt.Errorf("q-value must be between 0 and 1, got %f", q)
	}

	// Round to 3 decimal places as per RFC
	q = float64(int(q*1000)) / 1000

	return q, nil
}

// isValidMediaType validates a media type format
func isValidMediaType(mediaType string) bool {
	// Basic validation: must contain a slash
	if !strings.Contains(mediaType, "/") {
		return false
	}

	// Split into type and subtype
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return false
	}

	mainType := parts[0]
	subType := parts[1]

	// Validate main type
	if mainType == "" || (!isValidToken(mainType) && mainType != "*") {
		return false
	}

	// Validate subtype
	if subType == "" || (!isValidToken(subType) && subType != "*") {
		return false
	}

	return true
}

// isValidToken checks if a string is a valid HTTP token
func isValidToken(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if !isTokenChar(r) {
			return false
		}
	}

	return true
}

// isTokenChar checks if a rune is a valid token character
func isTokenChar(r rune) bool {
	// Token characters as per RFC 7230
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '!' || r == '#' || r == '$' || r == '%' || r == '&' ||
		r == '\'' || r == '*' || r == '+' || r == '-' || r == '.' ||
		r == '^' || r == '_' || r == '`' || r == '|' || r == '~'
}

// sortAcceptTypes sorts accept types by quality factor (highest first)
func sortAcceptTypes(types []AcceptType) {
	// Stable sort to preserve order of equal quality types
	for i := 1; i < len(types); i++ {
		j := i
		for j > 0 && types[j].Quality > types[j-1].Quality {
			types[j], types[j-1] = types[j-1], types[j]
			j--
		}
	}
}

// ParseMediaType parses a media type with parameters (e.g., from Content-Type header)
func ParseMediaType(mediaType string) (string, map[string]string, error) {
	params := make(map[string]string)

	// Split by semicolon
	parts := strings.Split(mediaType, ";")
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty media type")
	}

	// First part is the media type
	baseType := strings.TrimSpace(parts[0])
	if !isValidMediaType(baseType) {
		return "", nil, fmt.Errorf("invalid media type: %s", baseType)
	}

	// Parse parameters
	for i := 1; i < len(parts); i++ {
		param := strings.TrimSpace(parts[i])
		if param == "" {
			continue
		}

		// Split by equals
		paramParts := strings.SplitN(param, "=", 2)
		if len(paramParts) != 2 {
			continue // Skip invalid parameters
		}

		key := strings.TrimSpace(paramParts[0])
		value := strings.TrimSpace(paramParts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"")

		params[key] = value
	}

	return baseType, params, nil
}

// FormatMediaType formats a media type with parameters
func FormatMediaType(mediaType string, params map[string]string) string {
	if len(params) == 0 {
		return mediaType
	}

	var parts []string
	parts = append(parts, mediaType)

	// Add parameters
	for key, value := range params {
		// Quote value if it contains special characters
		if needsQuoting(value) {
			parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, value))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(parts, "; ")
}

// needsQuoting checks if a parameter value needs quoting
func needsQuoting(value string) bool {
	for _, r := range value {
		if !isTokenChar(r) {
			return true
		}
	}
	return false
}

// MatchMediaTypes checks if two media types match (considering wildcards)
func MatchMediaTypes(type1, type2 string) bool {
	// Exact match
	if type1 == type2 {
		return true
	}

	// Parse both types
	parts1 := strings.Split(type1, "/")
	parts2 := strings.Split(type2, "/")

	if len(parts1) != 2 || len(parts2) != 2 {
		return false
	}

	// Check for wildcards
	if parts1[0] == "*" || parts2[0] == "*" {
		return true
	}

	// Check main type match with subtype wildcard
	if parts1[0] == parts2[0] {
		if parts1[1] == "*" || parts2[1] == "*" {
			return true
		}
	}

	return false
}