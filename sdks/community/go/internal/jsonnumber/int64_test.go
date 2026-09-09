package jsonnumber

import "testing"

func TestInt64RequiresNumericJSON(t *testing.T) {
	for _, raw := range []string{"", "null", "true", "[]", "{}", `"1"`, "1/1", "0x10", "+1", "01", "1 2", "0e+", "NaN"} {
		if _, err := Int64([]byte(raw)); err == nil {
			t.Errorf("accepted nonnumeric JSON token %q", raw)
		}
	}
}
