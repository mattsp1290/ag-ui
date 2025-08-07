package transport

import (
	"errors"
	"testing"
)

func TestTypedConfigurationErrors(t *testing.T) {
	t.Run("string_config_error", func(t *testing.T) {
		err := NewStringConfigError("hostname", "invalid_host", "hostname cannot contain underscores")

		expectedMsg := "configuration error for field hostname (value: invalid_host): hostname cannot contain underscores"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}

		if err.Field != "hostname" {
			t.Errorf("Expected field %q, got %q", "hostname", err.Field)
		}

		if err.Value.Value != "invalid_host" {
			t.Errorf("Expected value %q, got %q", "invalid_host", err.Value.Value)
		}
	})

	t.Run("int_config_error", func(t *testing.T) {
		err := NewIntConfigError("timeout", -5, "timeout must be positive")

		expectedMsg := "configuration error for field timeout (value: -5): timeout must be positive"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}

		if err.Value.Value != -5 {
			t.Errorf("Expected value %d, got %d", -5, err.Value.Value)
		}
	})

	t.Run("bool_config_error", func(t *testing.T) {
		err := NewBoolConfigError("enabled", false, "feature must be enabled")

		expectedMsg := "configuration error for field enabled (value: false): feature must be enabled"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("float_config_error", func(t *testing.T) {
		err := NewFloatConfigError("ratio", 1.5, "ratio must be between 0 and 1")

		expectedMsg := "configuration error for field ratio (value: 1.5): ratio must be between 0 and 1"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("nil_config_error", func(t *testing.T) {
		err := NewNilConfigError("required_field", "field is required but was nil")

		expectedMsg := "configuration error for field required_field (value: <nil>): field is required but was nil"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("generic_config_error", func(t *testing.T) {
		complexValue := map[string]interface{}{"key": "value", "number": 42}
		err := NewGenericConfigError("complex_config", complexValue, "invalid complex configuration")

		if err.Value.Value == nil {
			t.Error("Expected non-nil value in generic config error")
		}
	})

	t.Run("validate_error_value", func(t *testing.T) {
		tests := []struct {
			name     string
			input    interface{}
			expected string
		}{
			{"string", "test", "test"},
			{"int", 42, "42"},
			{"bool_true", true, "true"},
			{"bool_false", false, "false"},
			{"float64", 3.14, "3.14"},
			{"nil", nil, "<nil>"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				errorValue := ValidateErrorValue(tt.input)
				if errorValue.ErrorString() != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, errorValue.ErrorString())
				}
			})
		}
	})

	t.Run("create_typed_config_error", func(t *testing.T) {
		err := CreateTypedConfigError("port", 8080, "port already in use")

		if !IsConfigurationError(err) {
			t.Error("Expected error to be recognized as configuration error")
		}

		field := GetConfigurationErrorField(err)
		if field != "port" {
			t.Errorf("Expected field %q, got %q", "port", field)
		}

		value := GetConfigurationErrorValue(err)
		if value != 8080 {
			t.Errorf("Expected value %d, got %v", 8080, value)
		}
	})

	t.Run("backward_compatibility", func(t *testing.T) {
		// Test that legacy ConfigurationError still works
		legacyErr := NewLegacyConfigurationError("timeout", -1, "timeout must be positive")

		expectedMsg := "configuration error for field timeout (value: -1): timeout must be positive"
		if legacyErr.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, legacyErr.Error())
		}

		if !IsConfigurationError(legacyErr) {
			t.Error("Expected legacy error to be recognized as configuration error")
		}

		// Test ConfigError alias
		aliasErr := NewConfigError("buffer_size", 0, "buffer size cannot be zero")
		if !IsConfigurationError(aliasErr) {
			t.Error("Expected alias error to be recognized as configuration error")
		}
	})

	t.Run("error_helper_functions", func(t *testing.T) {
		// Test with typed error
		typedErr := NewStringConfigError("host", "localhost", "invalid host")
		if field := GetConfigurationErrorField(typedErr); field != "host" {
			t.Errorf("Expected field %q, got %q", "host", field)
		}
		if value := GetConfigurationErrorValue(typedErr); value != "localhost" {
			t.Errorf("Expected value %q, got %v", "localhost", value)
		}

		// Test with legacy error
		legacyErr := NewLegacyConfigurationError("port", 3000, "port in use")
		if field := GetConfigurationErrorField(legacyErr); field != "port" {
			t.Errorf("Expected field %q, got %q", "port", field)
		}
		if value := GetConfigurationErrorValue(legacyErr); value != 3000 {
			t.Errorf("Expected value %d, got %v", 3000, value)
		}

		// Test with non-configuration error
		nonConfigErr := errors.New("some other error")
		if IsConfigurationError(nonConfigErr) {
			t.Error("Expected non-config error to not be recognized as configuration error")
		}
		if field := GetConfigurationErrorField(nonConfigErr); field != "" {
			t.Errorf("Expected empty field for non-config error, got %q", field)
		}
		if value := GetConfigurationErrorValue(nonConfigErr); value != nil {
			t.Errorf("Expected nil value for non-config error, got %v", value)
		}
	})
}
