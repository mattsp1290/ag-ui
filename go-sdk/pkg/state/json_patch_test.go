package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestJSONPatchOpValidate tests validation of individual JSON Patch operations
func TestJSONPatchOpValidate(t *testing.T) {
	tests := []struct {
		name    string
		op      JSONPatchOperation
		wantErr bool
		errMsg  string
	}{
		// Valid operations
		{
			name: "valid add operation",
			op: JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  "/foo",
				Value: "bar",
			},
			wantErr: false,
		},
		{
			name: "valid remove operation",
			op: JSONPatchOperation{
				Op:   JSONPatchOpRemove,
				Path: "/foo",
			},
			wantErr: false,
		},
		{
			name: "valid replace operation",
			op: JSONPatchOperation{
				Op:    JSONPatchOpReplace,
				Path:  "/foo",
				Value: "bar",
			},
			wantErr: false,
		},
		{
			name: "valid move operation",
			op: JSONPatchOperation{
				Op:   JSONPatchOpMove,
				Path: "/foo",
				From: "/bar",
			},
			wantErr: false,
		},
		{
			name: "valid copy operation",
			op: JSONPatchOperation{
				Op:   JSONPatchOpCopy,
				Path: "/foo",
				From: "/bar",
			},
			wantErr: false,
		},
		{
			name: "valid test operation",
			op: JSONPatchOperation{
				Op:    JSONPatchOpTest,
				Path:  "/foo",
				Value: "bar",
			},
			wantErr: false,
		},
		// Invalid operations
		{
			name: "invalid operation type",
			op: JSONPatchOperation{
				Op:   "invalid",
				Path: "/foo",
			},
			wantErr: true,
			errMsg:  "invalid operation type: invalid",
		},
		{
			name: "missing path",
			op: JSONPatchOperation{
				Op: JSONPatchOpAdd,
				// Path is empty string, which is valid (refers to root)
			},
			wantErr: false,
		},
		{
			name: "invalid path format",
			op: JSONPatchOperation{
				Op:   JSONPatchOpAdd,
				Path: "foo",
			},
			wantErr: true,
			errMsg:  "invalid path: JSON Pointer must start with '/' or be empty",
		},
		{
			name: "move operation missing from",
			op: JSONPatchOperation{
				Op:   JSONPatchOpMove,
				Path: "/foo",
			},
			wantErr: true,
			errMsg:  "from field is required for move operation",
		},
		{
			name: "copy operation missing from",
			op: JSONPatchOperation{
				Op:   JSONPatchOpCopy,
				Path: "/foo",
			},
			wantErr: true,
			errMsg:  "from field is required for copy operation",
		},
		{
			name: "move operation with same path and from",
			op: JSONPatchOperation{
				Op:   JSONPatchOpMove,
				Path: "/foo",
				From: "/foo",
			},
			wantErr: true,
			errMsg:  "move operation cannot have the same path and from",
		},
		{
			name: "invalid from path",
			op: JSONPatchOperation{
				Op:   JSONPatchOpMove,
				Path: "/foo",
				From: "bar",
			},
			wantErr: true,
			errMsg:  "invalid from path: JSON Pointer must start with '/' or be empty",
		},
		{
			name: "add with null value",
			op: JSONPatchOperation{
				Op:    JSONPatchOpAdd,
				Path:  "/foo",
				Value: nil,
			},
			wantErr: false,
		},
		{
			name: "empty root path",
			op: JSONPatchOperation{
				Op:    JSONPatchOpReplace,
				Path:  "",
				Value: "new",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestJSONPatchValidate tests validation of JSON Patch collections
func TestJSONPatchValidate(t *testing.T) {
	tests := []struct {
		name    string
		patch   JSONPatch
		wantErr bool
	}{
		{
			name: "valid patch with multiple operations",
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/foo", Value: "bar"},
				JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/baz"},
			},
			wantErr: false,
		},
		{
			name:    "empty patch",
			patch:   JSONPatch{},
			wantErr: false,
		},
		{
			name: "patch with invalid operation",
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/foo", Value: "bar"},
				JSONPatchOperation{Op: "invalid", Path: "/baz"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.patch.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestApplyAdd tests the add operation
func TestApplyAdd(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		path     string
		value    interface{}
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "add to object",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/baz",
			value:    "qux",
			want:     map[string]interface{}{"foo": "bar", "baz": "qux"},
			wantErr:  false,
		},
		{
			name:     "add to nested object",
			document: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			path:     "/foo/qux",
			value:    "quux",
			want:     map[string]interface{}{"foo": map[string]interface{}{"bar": "baz", "qux": "quux"}},
			wantErr:  false,
		},
		{
			name:     "add to array - append",
			document: []interface{}{"foo", "bar"},
			path:     "/-",
			value:    "baz",
			want:     []interface{}{"foo", "bar", "baz"},
			wantErr:  false,
		},
		{
			name:     "add to array - insert",
			document: []interface{}{"foo", "bar"},
			path:     "/1",
			value:    "baz",
			want:     []interface{}{"foo", "baz", "bar"},
			wantErr:  false,
		},
		{
			name:     "add to array - index 0",
			document: []interface{}{"foo", "bar"},
			path:     "/0",
			value:    "baz",
			want:     []interface{}{"baz", "foo", "bar"},
			wantErr:  false,
		},
		{
			name:     "replace root document",
			document: map[string]interface{}{"foo": "bar"},
			path:     "",
			value:    "new",
			want:     "new",
			wantErr:  false,
		},
		{
			name:     "add to empty object",
			document: map[string]interface{}{},
			path:     "/foo",
			value:    "bar",
			want:     map[string]interface{}{"foo": "bar"},
			wantErr:  false,
		},
		{
			name:     "add complex value",
			document: map[string]interface{}{},
			path:     "/foo",
			value:    map[string]interface{}{"bar": []interface{}{1, 2, 3}},
			want:     map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{1.0, 2.0, 3.0}}},
			wantErr:  false,
		},
		{
			name:     "add null value",
			document: map[string]interface{}{},
			path:     "/foo",
			value:    nil,
			want:     map[string]interface{}{"foo": nil},
			wantErr:  false,
		},
		{
			name:     "add to non-existent parent",
			document: map[string]interface{}{},
			path:     "/foo/bar",
			value:    "baz",
			wantErr:  true,
		},
		{
			name:     "add to non-object/non-array",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo/baz",
			value:    "qux",
			wantErr:  true,
		},
		{
			name:     "add with invalid array index",
			document: []interface{}{"foo", "bar"},
			path:     "/invalid",
			value:    "baz",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyAdd(tt.document, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyAdd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyAdd() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyRemove tests the remove operation
func TestApplyRemove(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		path     string
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "remove from object",
			document: map[string]interface{}{"foo": "bar", "baz": "qux"},
			path:     "/foo",
			want:     map[string]interface{}{"baz": "qux"},
			wantErr:  false,
		},
		{
			name:     "remove from nested object",
			document: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz", "qux": "quux"}},
			path:     "/foo/bar",
			want:     map[string]interface{}{"foo": map[string]interface{}{"qux": "quux"}},
			wantErr:  false,
		},
		{
			name:     "remove from array",
			document: []interface{}{"foo", "bar", "baz"},
			path:     "/1",
			want:     []interface{}{"foo", "baz"},
			wantErr:  false,
		},
		{
			name:     "remove last element from array",
			document: []interface{}{"foo", "bar", "baz"},
			path:     "/2",
			want:     []interface{}{"foo", "bar"},
			wantErr:  false,
		},
		{
			name:     "remove first element from array",
			document: []interface{}{"foo", "bar", "baz"},
			path:     "/0",
			want:     []interface{}{"bar", "baz"},
			wantErr:  false,
		},
		{
			name:     "remove root document",
			document: map[string]interface{}{"foo": "bar"},
			path:     "",
			wantErr:  true,
		},
		{
			name:     "remove non-existent key",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/baz",
			wantErr:  true,
		},
		{
			name:     "remove from non-existent parent",
			document: map[string]interface{}{},
			path:     "/foo/bar",
			wantErr:  true,
		},
		{
			name:     "remove with array index out of bounds",
			document: []interface{}{"foo", "bar"},
			path:     "/2",
			wantErr:  true,
		},
		{
			name:     "remove from non-object/non-array",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo/bar",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyRemove(tt.document, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyRemove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyRemove() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyReplace tests the replace operation
func TestApplyReplace(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		path     string
		value    interface{}
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "replace in object",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo",
			value:    "baz",
			want:     map[string]interface{}{"foo": "baz"},
			wantErr:  false,
		},
		{
			name:     "replace in nested object",
			document: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			path:     "/foo/bar",
			value:    "qux",
			want:     map[string]interface{}{"foo": map[string]interface{}{"bar": "qux"}},
			wantErr:  false,
		},
		{
			name:     "replace in array",
			document: []interface{}{"foo", "bar", "baz"},
			path:     "/1",
			value:    "qux",
			want:     []interface{}{"foo", "qux", "baz"},
			wantErr:  false,
		},
		{
			name:     "replace root document",
			document: map[string]interface{}{"foo": "bar"},
			path:     "",
			value:    []interface{}{"new"},
			want:     []interface{}{"new"},
			wantErr:  false,
		},
		{
			name:     "replace with complex value",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo",
			value:    map[string]interface{}{"nested": true},
			want:     map[string]interface{}{"foo": map[string]interface{}{"nested": true}},
			wantErr:  false,
		},
		{
			name:     "replace with null",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo",
			value:    nil,
			want:     map[string]interface{}{"foo": nil},
			wantErr:  false,
		},
		{
			name:     "replace non-existent path",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/baz",
			value:    "qux",
			wantErr:  true,
		},
		{
			name:     "replace in non-existent parent",
			document: map[string]interface{}{},
			path:     "/foo/bar",
			value:    "baz",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyReplace(tt.document, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyReplace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyReplace() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyMove tests the move operation
func TestApplyMove(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		from     string
		path     string
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "move in object",
			document: map[string]interface{}{"foo": "bar", "baz": "qux"},
			from:     "/foo",
			path:     "/qux",
			want:     map[string]interface{}{"baz": "qux", "qux": "bar"},
			wantErr:  false,
		},
		{
			name:     "move to existing key",
			document: map[string]interface{}{"foo": "bar", "baz": "qux"},
			from:     "/foo",
			path:     "/baz",
			want:     map[string]interface{}{"baz": "bar"},
			wantErr:  false,
		},
		{
			name:     "move in array",
			document: []interface{}{"foo", "bar", "baz"},
			from:     "/0",
			path:     "/2",
			want:     []interface{}{"bar", "baz", "foo"},
			wantErr:  false,
		},
		{
			name:     "move from array to object",
			document: map[string]interface{}{"arr": []interface{}{"foo", "bar"}},
			from:     "/arr/0",
			path:     "/moved",
			want:     map[string]interface{}{"arr": []interface{}{"bar"}, "moved": "foo"},
			wantErr:  false,
		},
		{
			name:     "move complex value",
			document: map[string]interface{}{"foo": map[string]interface{}{"nested": true}},
			from:     "/foo",
			path:     "/bar",
			want:     map[string]interface{}{"bar": map[string]interface{}{"nested": true}},
			wantErr:  false,
		},
		{
			name:     "move from non-existent path",
			document: map[string]interface{}{"foo": "bar"},
			from:     "/baz",
			path:     "/qux",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyMove(tt.document, tt.from, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyMove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyMove() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyCopy tests the copy operation
func TestApplyCopy(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		from     string
		path     string
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "copy in object",
			document: map[string]interface{}{"foo": "bar"},
			from:     "/foo",
			path:     "/baz",
			want:     map[string]interface{}{"foo": "bar", "baz": "bar"},
			wantErr:  false,
		},
		{
			name:     "copy to existing key",
			document: map[string]interface{}{"foo": "bar", "baz": "qux"},
			from:     "/foo",
			path:     "/baz",
			want:     map[string]interface{}{"foo": "bar", "baz": "bar"},
			wantErr:  false,
		},
		{
			name:     "copy complex value",
			document: map[string]interface{}{"foo": map[string]interface{}{"nested": true}},
			from:     "/foo",
			path:     "/bar",
			want:     map[string]interface{}{"foo": map[string]interface{}{"nested": true}, "bar": map[string]interface{}{"nested": true}},
			wantErr:  false,
		},
		{
			name:     "copy from array to object",
			document: map[string]interface{}{"arr": []interface{}{"foo", "bar"}},
			from:     "/arr/0",
			path:     "/copied",
			want:     map[string]interface{}{"arr": []interface{}{"foo", "bar"}, "copied": "foo"},
			wantErr:  false,
		},
		{
			name:     "copy to array append",
			document: []interface{}{"foo", "bar"},
			from:     "/0",
			path:     "/-",
			want:     []interface{}{"foo", "bar", "foo"},
			wantErr:  false,
		},
		{
			name:     "copy from non-existent path",
			document: map[string]interface{}{"foo": "bar"},
			from:     "/baz",
			path:     "/qux",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyCopy(tt.document, tt.from, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyCopy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApplyTest tests the test operation
func TestApplyTest(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		path     string
		value    interface{}
		wantErr  bool
	}{
		{
			name:     "test string value - pass",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo",
			value:    "bar",
			wantErr:  false,
		},
		{
			name:     "test string value - fail",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/foo",
			value:    "baz",
			wantErr:  true,
		},
		{
			name:     "test number value",
			document: map[string]interface{}{"num": float64(42)},
			path:     "/num",
			value:    float64(42),
			wantErr:  false,
		},
		{
			name:     "test boolean value",
			document: map[string]interface{}{"bool": true},
			path:     "/bool",
			value:    true,
			wantErr:  false,
		},
		{
			name:     "test null value",
			document: map[string]interface{}{"null": nil},
			path:     "/null",
			value:    nil,
			wantErr:  false,
		},
		{
			name:     "test array value",
			document: map[string]interface{}{"arr": []interface{}{"foo", "bar"}},
			path:     "/arr",
			value:    []interface{}{"foo", "bar"},
			wantErr:  false,
		},
		{
			name:     "test object value",
			document: map[string]interface{}{"obj": map[string]interface{}{"nested": true}},
			path:     "/obj",
			value:    map[string]interface{}{"nested": true},
			wantErr:  false,
		},
		{
			name:     "test nested value",
			document: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			path:     "/foo/bar",
			value:    "baz",
			wantErr:  false,
		},
		{
			name:     "test array element",
			document: []interface{}{"foo", "bar", "baz"},
			path:     "/1",
			value:    "bar",
			wantErr:  false,
		},
		{
			name:     "test root document",
			document: "root",
			path:     "",
			value:    "root",
			wantErr:  false,
		},
		{
			name:     "test non-existent path",
			document: map[string]interface{}{"foo": "bar"},
			path:     "/baz",
			value:    "qux",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := applyTest(tt.document, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyTest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestJSONPatchApply tests applying a full JSON Patch
func TestJSONPatchApply(t *testing.T) {
	tests := []struct {
		name     string
		document interface{}
		patch    JSONPatch
		want     interface{}
		wantErr  bool
	}{
		{
			name:     "multiple operations",
			document: map[string]interface{}{"foo": "bar"},
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/baz", Value: "qux"},
				JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/foo", Value: "replaced"},
				JSONPatchOperation{Op: JSONPatchOpTest, Path: "/baz", Value: "qux"},
			},
			want:    map[string]interface{}{"foo": "replaced", "baz": "qux"},
			wantErr: false,
		},
		{
			name:     "array manipulation",
			document: map[string]interface{}{"arr": []interface{}{"a", "b", "c"}},
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/arr/-", Value: "d"},
				JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/arr/1"},
				JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/arr/0", Value: "A"},
			},
			want:    map[string]interface{}{"arr": []interface{}{"A", "c", "d"}},
			wantErr: false,
		},
		{
			name:     "move and copy operations",
			document: map[string]interface{}{"foo": "bar", "baz": []interface{}{1, 2, 3}},
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpCopy, From: "/baz/0", Path: "/first"},
				JSONPatchOperation{Op: JSONPatchOpMove, From: "/foo", Path: "/moved"},
			},
			want:    map[string]interface{}{"baz": []interface{}{float64(1), float64(2), float64(3)}, "first": float64(1), "moved": "bar"},
			wantErr: false,
		},
		{
			name:     "failed test stops execution",
			document: map[string]interface{}{"foo": "bar"},
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpTest, Path: "/foo", Value: "wrong"},
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/should_not_add", Value: "value"},
			},
			wantErr: true,
		},
		{
			name:     "invalid operation stops execution",
			document: map[string]interface{}{"foo": "bar"},
			patch: JSONPatch{
				JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/baz", Value: "qux"},
				JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/non_existent"},
			},
			wantErr: true,
		},
		{
			name:     "empty patch",
			document: map[string]interface{}{"foo": "bar"},
			patch:    JSONPatch{},
			want:     map[string]interface{}{"foo": "bar"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.patch.Apply(tt.document)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(tt.want)
				t.Errorf("Apply() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestParseJSONPointer tests JSON Pointer parsing
func TestParseJSONPointer(t *testing.T) {
	tests := []struct {
		name    string
		pointer string
		want    []string
	}{
		{
			name:    "empty pointer",
			pointer: "",
			want:    []string{},
		},
		{
			name:    "root pointer",
			pointer: "/",
			want:    []string{""},
		},
		{
			name:    "simple path",
			pointer: "/foo",
			want:    []string{"foo"},
		},
		{
			name:    "nested path",
			pointer: "/foo/bar/baz",
			want:    []string{"foo", "bar", "baz"},
		},
		{
			name:    "path with escaped characters",
			pointer: "/foo~0bar/baz~1qux",
			want:    []string{"foo~bar", "baz/qux"},
		},
		{
			name:    "path with numbers",
			pointer: "/0/1/2",
			want:    []string{"0", "1", "2"},
		},
		{
			name:    "path with special characters",
			pointer: "/foo bar/baz-qux/hello_world",
			want:    []string{"foo bar", "baz-qux", "hello_world"},
		},
		{
			name:    "invalid pointer without leading slash",
			pointer: "foo",
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJSONPointer(tt.pointer)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseJSONPointer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUnescapeJSONPointer tests JSON Pointer unescaping
func TestUnescapeJSONPointer(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "no escape sequences",
			token: "foo",
			want:  "foo",
		},
		{
			name:  "escape tilde",
			token: "foo~0bar",
			want:  "foo~bar",
		},
		{
			name:  "escape forward slash",
			token: "foo~1bar",
			want:  "foo/bar",
		},
		{
			name:  "multiple escapes",
			token: "~0~1~0~1",
			want:  "~/~/",
		},
		{
			name:  "escape sequence at start",
			token: "~0foo",
			want:  "~foo",
		},
		{
			name:  "escape sequence at end",
			token: "foo~1",
			want:  "foo/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unescapeJSONPointer(tt.token)
			if got != tt.want {
				t.Errorf("unescapeJSONPointer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateJSONPointer tests JSON Pointer validation
func TestValidateJSONPointer(t *testing.T) {
	tests := []struct {
		name    string
		pointer string
		wantErr bool
	}{
		{
			name:    "empty pointer",
			pointer: "",
			wantErr: false,
		},
		{
			name:    "valid pointer",
			pointer: "/foo/bar",
			wantErr: false,
		},
		{
			name:    "root pointer",
			pointer: "/",
			wantErr: false,
		},
		{
			name:    "invalid pointer without slash",
			pointer: "foo",
			wantErr: true,
		},
		{
			name:    "pointer with escaped characters",
			pointer: "/foo~0bar~1baz",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONPointer(tt.pointer)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJSONPointer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseArrayIndex tests array index parsing
func TestParseArrayIndex(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		length     int
		wantIdx    int
		wantAppend bool
		wantErr    bool
	}{
		{
			name:       "append token",
			token:      "-",
			length:     3,
			wantIdx:    3,
			wantAppend: true,
			wantErr:    false,
		},
		{
			name:       "valid index 0",
			token:      "0",
			length:     3,
			wantIdx:    0,
			wantAppend: false,
			wantErr:    false,
		},
		{
			name:       "valid index middle",
			token:      "1",
			length:     3,
			wantIdx:    1,
			wantAppend: false,
			wantErr:    false,
		},
		{
			name:       "valid index last",
			token:      "2",
			length:     3,
			wantIdx:    2,
			wantAppend: false,
			wantErr:    false,
		},
		{
			name:    "invalid token",
			token:   "abc",
			length:  3,
			wantErr: true,
		},
		{
			name:    "negative index",
			token:   "-1",
			length:  3,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, isAppend, err := parseArrayIndex(tt.token, tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArrayIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if idx != tt.wantIdx {
					t.Errorf("parseArrayIndex() idx = %v, want %v", idx, tt.wantIdx)
				}
				if isAppend != tt.wantAppend {
					t.Errorf("parseArrayIndex() isAppend = %v, want %v", isAppend, tt.wantAppend)
				}
			}
		})
	}
}

// TestDeepCopy tests the deep copy functionality
func TestDeepCopy(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "nil value",
			value: nil,
		},
		{
			name:  "string value",
			value: "test",
		},
		{
			name:  "number value",
			value: 42.5,
		},
		{
			name:  "boolean value",
			value: true,
		},
		{
			name:  "array value",
			value: []interface{}{"a", "b", "c"},
		},
		{
			name:  "object value",
			value: map[string]interface{}{"foo": "bar", "baz": "qux"},
		},
		{
			name: "nested structure",
			value: map[string]interface{}{
				"array": []interface{}{1, 2, 3},
				"object": map[string]interface{}{
					"nested": true,
					"deep": map[string]interface{}{
						"value": "test",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepCopy(tt.value)
			if !reflect.DeepEqual(got, tt.value) {
				t.Errorf("deepCopy() = %v, want %v", got, tt.value)
			}

			// Verify it's a true copy by modifying the original (where applicable)
			switch v := tt.value.(type) {
			case map[string]interface{}:
				v["modified"] = true
				if _, ok := got.(map[string]interface{})["modified"]; ok {
					t.Error("deepCopy() did not create independent copy of map")
				}
			case []interface{}:
				if len(v) > 0 {
					v[0] = "modified"
					if got.([]interface{})[0] == "modified" {
						t.Error("deepCopy() did not create independent copy of slice")
					}
				}
			}
		})
	}
}

// BenchmarkJSONPatchApply benchmarks the Apply operation
func BenchmarkJSONPatchApply(b *testing.B) {
	document := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"id": 1, "name": "Alice"},
			map[string]interface{}{"id": 2, "name": "Bob"},
			map[string]interface{}{"id": 3, "name": "Charlie"},
		},
		"settings": map[string]interface{}{
			"theme":         "dark",
			"notifications": true,
		},
	}

	patch := JSONPatch{
		JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/users/-", Value: map[string]interface{}{"id": 4, "name": "David"}},
		JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/settings/theme", Value: "light"},
		JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/users/0"},
		JSONPatchOperation{Op: JSONPatchOpCopy, From: "/users/0", Path: "/backup"},
		JSONPatchOperation{Op: JSONPatchOpTest, Path: "/settings/notifications", Value: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := patch.Apply(document)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONPointerParse benchmarks JSON Pointer parsing
func BenchmarkJSONPointerParse(b *testing.B) {
	pointers := []string{
		"/users/0/name",
		"/deeply/nested/object/with/many/levels",
		"/escaped~0chars~1here",
		"/simple",
		"/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pointer := range pointers {
			_ = parseJSONPointer(pointer)
		}
	}
}

// BenchmarkDeepCopy benchmarks the deep copy operation
func BenchmarkDeepCopy(b *testing.B) {
	value := map[string]interface{}{
		"array": []interface{}{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"deep": map[string]interface{}{
				"value": "test",
				"more":  []interface{}{"a", "b", "c"},
			},
		},
		"string": "hello world",
		"number": 42.5,
		"bool":   true,
		"null":   nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepCopy(value)
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("operations on empty document", func(t *testing.T) {
		var doc interface{}
		patch := JSONPatch{
			JSONPatchOperation{Op: JSONPatchOpReplace, Path: "", Value: map[string]interface{}{"new": "doc"}},
		}
		got, err := patch.Apply(doc)
		if err != nil {
			t.Errorf("Apply() error = %v", err)
		}
		want := map[string]interface{}{"new": "doc"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Apply() = %v, want %v", got, want)
		}
	})

	t.Run("operations on different types", func(t *testing.T) {
		// Start with a string, replace with object
		doc := "string"
		patch := JSONPatch{
			JSONPatchOperation{Op: JSONPatchOpReplace, Path: "", Value: map[string]interface{}{"type": "object"}},
		}
		got, err := patch.Apply(doc)
		if err != nil {
			t.Errorf("Apply() error = %v", err)
		}
		want := map[string]interface{}{"type": "object"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Apply() = %v, want %v", got, want)
		}
	})

	t.Run("array operations with different types", func(t *testing.T) {
		doc := []interface{}{
			"string",
			42.0,
			true,
			nil,
			map[string]interface{}{"nested": true},
			[]interface{}{1, 2, 3},
		}
		patch := JSONPatch{
			JSONPatchOperation{Op: JSONPatchOpTest, Path: "/0", Value: "string"},
			JSONPatchOperation{Op: JSONPatchOpTest, Path: "/1", Value: 42.0},
			JSONPatchOperation{Op: JSONPatchOpTest, Path: "/2", Value: true},
			JSONPatchOperation{Op: JSONPatchOpTest, Path: "/3", Value: nil},
		}
		_, err := patch.Apply(doc)
		if err != nil {
			t.Errorf("Apply() error = %v", err)
		}
	})

	t.Run("deeply nested operations", func(t *testing.T) {
		doc := map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": map[string]interface{}{
						"d": map[string]interface{}{
							"e": "deep",
						},
					},
				},
			},
		}
		patch := JSONPatch{
			JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/a/b/c/d/e", Value: "replaced"},
			JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/a/b/c/d/f", Value: "added"},
			JSONPatchOperation{Op: JSONPatchOpCopy, From: "/a/b/c/d", Path: "/copied"},
		}
		got, err := patch.Apply(doc)
		if err != nil {
			t.Errorf("Apply() error = %v", err)
		}

		// Verify the operations
		result := got.(map[string]interface{})
		deepValue := result["a"].(map[string]interface{})["b"].(map[string]interface{})["c"].(map[string]interface{})["d"].(map[string]interface{})["e"]
		if deepValue != "replaced" {
			t.Errorf("Deep replace failed, got %v", deepValue)
		}
	})
}

// TestConcurrentOperations tests that operations maintain consistency
func TestConcurrentOperations(t *testing.T) {
	// This test verifies that each Apply creates a new document
	// and doesn't modify the original
	original := map[string]interface{}{
		"counter": 0,
		"list":    []interface{}{"a", "b", "c"},
	}

	patch1 := JSONPatch{
		JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/counter", Value: 1},
		JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/list/-", Value: "d"},
	}

	patch2 := JSONPatch{
		JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/counter", Value: 2},
		JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/list/0"},
	}

	// Apply patches
	result1, err := patch1.Apply(original)
	if err != nil {
		t.Fatalf("patch1.Apply() error = %v", err)
	}

	result2, err := patch2.Apply(original)
	if err != nil {
		t.Fatalf("patch2.Apply() error = %v", err)
	}

	// Verify original is unchanged
	if original["counter"] != 0 {
		t.Error("Original document was modified")
	}
	if len(original["list"].([]interface{})) != 3 {
		t.Error("Original list was modified")
	}

	// Verify results are independent
	if result1.(map[string]interface{})["counter"] != float64(1) {
		t.Error("Result1 counter incorrect")
	}
	if result2.(map[string]interface{})["counter"] != float64(2) {
		t.Error("Result2 counter incorrect")
	}
}

// TestJSONMarshalUnmarshal tests JSON marshaling/unmarshaling of patches
func TestJSONMarshalUnmarshal(t *testing.T) {
	patch := JSONPatch{
		JSONPatchOperation{Op: JSONPatchOpAdd, Path: "/foo", Value: "bar"},
		JSONPatchOperation{Op: JSONPatchOpRemove, Path: "/baz"},
		JSONPatchOperation{Op: JSONPatchOpReplace, Path: "/qux", Value: 42},
		JSONPatchOperation{Op: JSONPatchOpMove, From: "/a", Path: "/b"},
		JSONPatchOperation{Op: JSONPatchOpCopy, From: "/c", Path: "/d"},
		JSONPatchOperation{Op: JSONPatchOpTest, Path: "/e", Value: true},
	}

	// Marshal to JSON
	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var unmarshaled JSONPatch
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify each operation
	if len(unmarshaled) != len(patch) {
		t.Fatalf("Unmarshaled patch has different length: got %d, want %d", len(unmarshaled), len(patch))
	}

	for i, op := range unmarshaled {
		if op.Op != patch[i].Op {
			t.Errorf("Operation %d: Op = %v, want %v", i, op.Op, patch[i].Op)
		}
		if op.Path != patch[i].Path {
			t.Errorf("Operation %d: Path = %v, want %v", i, op.Path, patch[i].Path)
		}
		if op.From != patch[i].From {
			t.Errorf("Operation %d: From = %v, want %v", i, op.From, patch[i].From)
		}
		// Value comparison needs special handling for numbers
		if op.Op == JSONPatchOpReplace && op.Path == "/qux" {
			if op.Value != float64(42) {
				t.Errorf("Operation %d: Value = %v, want %v", i, op.Value, float64(42))
			}
		} else if !reflect.DeepEqual(op.Value, patch[i].Value) {
			t.Errorf("Operation %d: Value = %v, want %v", i, op.Value, patch[i].Value)
		}
	}
}
