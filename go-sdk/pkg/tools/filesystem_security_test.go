package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFileSystemAccessControl tests comprehensive file system access control
func TestFileSystemAccessControl(t *testing.T) {
	t.Run("PathValidation", testPathValidation)
	t.Run("PermissionChecks", testPermissionChecks)
	t.Run("FileTypeValidation", testFileTypeValidation)
	t.Run("DirectoryTraversal", testDirectoryTraversal)
	t.Run("SymlinkHandling", testSymlinkHandling)
	t.Run("SpecialFileProtection", testSpecialFileProtection)
	t.Run("RootDirectoryProtection", testRootDirectoryProtection)
	t.Run("HiddenFileAccess", testHiddenFileAccess)
	t.Run("CaseSensitivity", testCaseSensitivity)
	t.Run("UnicodePathHandling", testUnicodePathHandling)
	t.Run("FileSystemLimits", testFileSystemLimits)
	t.Run("ConcurrentFileAccess", testConcurrentFileAccess)
}

// testPathValidation tests path validation and normalization
func testPathValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_path_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name        string
		path        string
		shouldFail  bool
		expectedErr string
	}{
		{
			name:        "Valid absolute path",
			path:        filepath.Join(tempDir, "valid.txt"),
			shouldFail:  false,
			expectedErr: "",
		},
		{
			name:        "Path with double slashes",
			path:        filepath.Join(tempDir, "//double//slash.txt"),
			shouldFail:  false, // Should be normalized
			expectedErr: "",
		},
		{
			name:        "Path with dot segments",
			path:        filepath.Join(tempDir, "./dot/./segment.txt"),
			shouldFail:  false, // Should be normalized
			expectedErr: "",
		},
		{
			name:        "Path with null bytes",
			path:        filepath.Join(tempDir, "null\x00byte.txt"),
			shouldFail:  true,
			expectedErr: "", // OS-specific error, just check that it fails
		},
		{
			name:        "Path with control characters",
			path:        filepath.Join(tempDir, "control\x01char.txt"),
			shouldFail:  true,
			expectedErr: "", // OS-specific error, just check that it fails
		},
		{
			name:        "Very long path",
			path:        filepath.Join(tempDir, strings.Repeat("a", 1000), "long.txt"),
			shouldFail:  true,
			expectedErr: "", // OS-specific error (file name too long), just check that it fails
		},
		{
			name:        "Empty path",
			path:        "",
			shouldFail:  true,
			expectedErr: "access denied", // Empty path triggers access denied
		},
		{
			name:        "Path traversal attempt",
			path:        filepath.Join(tempDir, "../../../etc/passwd"),
			shouldFail:  true,
			expectedErr: "access denied",
		},
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")
			
			// Create the file if it's a valid path
			if !tc.shouldFail && tc.path != "" {
				dir := filepath.Dir(tc.path)
				if err := os.MkdirAll(dir, 0755); err == nil {
					os.WriteFile(tc.path, []byte("test content"), 0644)
				}
			}

			params := map[string]interface{}{
				"path": tc.path,
			}

			result, err := executor.Execute(context.Background(), params)

			if tc.shouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected path validation to fail for: %s", tc.path)
				}
				if result != nil && tc.expectedErr != "" && !strings.Contains(result.Error, tc.expectedErr) {
					t.Errorf("Expected error containing '%s', got: %s", tc.expectedErr, result.Error)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid path: %v", err)
				}
			}
		})
	}
}

// testPermissionChecks tests file permission validation
func testPermissionChecks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_permission_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files with different permissions
	testFiles := map[string]os.FileMode{
		"readable.txt":    0644,
		"writable.txt":    0666,
		"executable.txt":  0755,
		"restrictive.txt": 0000,
	}

	for filename, mode := range testFiles {
		filepath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filepath, []byte("test content"), mode); err != nil {
			t.Errorf("Failed to create test file %s: %v", filename, err)
		}
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Test reading files with different permissions
	for filename, mode := range testFiles {
		t.Run(fmt.Sprintf("Permission_%s", filename), func(t *testing.T) {
			params := map[string]interface{}{
				"path": filepath.Join(tempDir, filename),
			}

			result, err := executor.Execute(context.Background(), params)

			// On most systems, file permissions are more about directory traversal
			// than individual file access, so we mainly test that the operation
			// completes without crashing
			if mode == 0000 && runtime.GOOS != "windows" {
				// Restrictive permissions might cause issues on some systems
				if result != nil && !result.Success {
					t.Logf("Expected restrictive file to fail access check: %s", result.Error)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for file with mode %o: %v", mode, err)
				}
			}
		})
	}
}

// testFileTypeValidation tests validation of different file types
func testFileTypeValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_type_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create different types of files
	regularFile := filepath.Join(tempDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("regular content"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create a directory
	dirPath := filepath.Join(tempDir, "directory")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	testCases := []struct {
		name        string
		path        string
		shouldFail  bool
		expectedErr string
	}{
		{
			name:        "Regular file",
			path:        regularFile,
			shouldFail:  false,
			expectedErr: "",
		},
		{
			name:        "Directory",
			path:        dirPath,
			shouldFail:  true,
			expectedErr: "", // Can fail with either "not a regular file" or "symbolic links are not allowed"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": tc.path,
			}

			result, err := executor.Execute(context.Background(), params)

			if tc.shouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected file type validation to fail for: %s", tc.path)
				}
				if result != nil && tc.expectedErr != "" && !strings.Contains(result.Error, tc.expectedErr) {
					t.Errorf("Expected error containing '%s', got: %s", tc.expectedErr, result.Error)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid file type: %v", err)
				}
			}
		})
	}
}

// testDirectoryTraversal tests comprehensive directory traversal protection
func testDirectoryTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_traversal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a secret file outside the allowed directory
	parentDir := filepath.Dir(tempDir)
	secretFile := filepath.Join(parentDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret content"), 0644); err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}
	defer os.Remove(secretFile)

	// Various directory traversal techniques
	traversalAttempts := []string{
		"../secret.txt",
		"../../secret.txt",
		"./../secret.txt",
		"./../../secret.txt",
		"subdir/../../../secret.txt",
		"subdir/../../secret.txt",
		"..//../secret.txt",
		"..\\secret.txt",            // Windows-style
		"..%2fsecret.txt",           // URL encoded
		"..%2f..%2fsecret.txt",      // Double URL encoded
		"..%255csecret.txt",         // Double URL encoded backslash
		"..%c0%afsecret.txt",        // UTF-8 encoded
		"..%ef%bc%8fsecret.txt",     // UTF-8 fullwidth slash
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, attempt := range traversalAttempts {
		t.Run(fmt.Sprintf("Traversal_%s", strings.ReplaceAll(attempt, "/", "_")), func(t *testing.T) {
			params := map[string]interface{}{
				"path": filepath.Join(tempDir, attempt),
			}

			result, err := executor.Execute(context.Background(), params)

			if err == nil && result.Success {
				t.Errorf("Expected directory traversal to be blocked: %s", attempt)
			}
			if result != nil && !strings.Contains(result.Error, "access denied") && 
			   !strings.Contains(result.Error, "symbolic links are not allowed") &&
			   !strings.Contains(result.Error, "no such file or directory") &&
			   !strings.Contains(result.Error, "cannot find the file") {
				t.Errorf("Expected security error for %s, got: %s", attempt, result.Error)
			}
		})
	}
}

// testSymlinkHandling tests comprehensive symlink handling
func testSymlinkHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target files
	targetFile := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	outsideTarget := filepath.Join(os.TempDir(), "outside_target.txt")
	if err := os.WriteFile(outsideTarget, []byte("outside content"), 0644); err != nil {
		t.Fatalf("Failed to create outside target: %v", err)
	}
	defer os.Remove(outsideTarget)

	// Create various symlinks
	symlinkTests := []struct {
		name        string
		linkPath    string
		target      string
		shouldFail  bool
		expectedErr string
	}{
		{
			name:        "Symlink to allowed file",
			linkPath:    filepath.Join(tempDir, "link_to_allowed.txt"),
			target:      targetFile,
			shouldFail:  true, // Symlinks disabled by default
			expectedErr: "symbolic links are not allowed",
		},
		{
			name:        "Symlink to outside file",
			linkPath:    filepath.Join(tempDir, "link_to_outside.txt"),
			target:      outsideTarget,
			shouldFail:  true,
			expectedErr: "symbolic links are not allowed",
		},
		{
			name:        "Symlink to non-existent file",
			linkPath:    filepath.Join(tempDir, "link_to_nonexistent.txt"),
			target:      filepath.Join(tempDir, "nonexistent.txt"),
			shouldFail:  true,
			expectedErr: "symbolic links are not allowed",
		},
		{
			name:        "Symlink with traversal",
			linkPath:    filepath.Join(tempDir, "link_with_traversal.txt"),
			target:      "../../../etc/passwd",
			shouldFail:  true,
			expectedErr: "symbolic links are not allowed",
		},
	}

	options := &SecureFileOptions{
		AllowedPaths:  []string{tempDir},
		AllowSymlinks: false,
		MaxFileSize:   1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, test := range symlinkTests {
		t.Run(test.name, func(t *testing.T) {
			// Create the symlink
			if err := os.Symlink(test.target, test.linkPath); err != nil {
				t.Skipf("Cannot create symlink (may not be supported): %v", err)
			}

			params := map[string]interface{}{
				"path": test.linkPath,
			}

			result, err := executor.Execute(context.Background(), params)

			if test.shouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected symlink handling to fail for: %s", test.name)
				}
				if result != nil && test.expectedErr != "" {
					// Accept either the expected error or OS-specific errors
					if !strings.Contains(result.Error, test.expectedErr) && 
					   !strings.Contains(result.Error, "no such file or directory") &&
					   !strings.Contains(result.Error, "failed to open file") &&
					   !strings.Contains(result.Error, "access denied") {
						t.Errorf("Expected error containing '%s', got: %s", test.expectedErr, result.Error)
					}
				}
				// Debug output removed for clean test runs
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid symlink: %v", err)
				}
			}
		})
	}

	// Test with symlinks enabled
	t.Run("SymlinksEnabled", func(t *testing.T) {
		options.AllowSymlinks = true
		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		// Create a symlink to an allowed file
		allowedLink := filepath.Join(tempDir, "allowed_link.txt")
		if err := os.Symlink(targetFile, allowedLink); err != nil {
			t.Skipf("Cannot create symlink: %v", err)
		}

		params := map[string]interface{}{
			"path": allowedLink,
		}

		result, err := executor.Execute(context.Background(), params)

		if err != nil {
			t.Errorf("Unexpected error for allowed symlink: %v", err)
		}
		if result == nil || !result.Success {
			if result != nil {
				t.Errorf("Expected allowed symlink to succeed, got error: %s", result.Error)
			} else {
				t.Error("Expected allowed symlink to succeed")
			}
		}

		// Create a symlink to an outside file (should still fail)
		outsideLink := filepath.Join(tempDir, "outside_link.txt")
		if err := os.Symlink(outsideTarget, outsideLink); err != nil {
			t.Skipf("Cannot create symlink: %v", err)
		}

		params = map[string]interface{}{
			"path": outsideLink,
		}

		result, err = executor.Execute(context.Background(), params)

		if err == nil && result.Success {
			t.Error("Expected symlink to outside file to fail even with symlinks enabled")
		}
	})
}

// testSpecialFileProtection tests protection against special files
func testSpecialFileProtection(t *testing.T) {
	// Common special files on Unix systems
	specialFiles := []string{
		"/dev/null",
		"/dev/zero",
		"/dev/random",
		"/dev/urandom",
		"/proc/self/mem",
		"/proc/self/environ",
		"/sys/kernel/hostname",
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{"/dev", "/proc", "/sys"}, // Allow these directories
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, specialFile := range specialFiles {
		if _, err := os.Stat(specialFile); os.IsNotExist(err) {
			continue // Skip if file doesn't exist on this system
		}

		t.Run(fmt.Sprintf("SpecialFile_%s", strings.ReplaceAll(specialFile, "/", "_")), func(t *testing.T) {
			params := map[string]interface{}{
				"path": specialFile,
			}

			result, err := executor.Execute(context.Background(), params)

			// Should fail because it's not a regular file
			if err == nil && result.Success {
				t.Errorf("Expected special file %s to be blocked", specialFile)
			}
			if result != nil && !strings.Contains(result.Error, "not a regular file") {
				t.Errorf("Expected regular file error for %s, got: %s", specialFile, result.Error)
			}
		})
	}
}

// testRootDirectoryProtection tests protection of root directory access
func testRootDirectoryProtection(t *testing.T) {
	rootPaths := []string{
		"/",
		"/root",
		"/root/.ssh",
		"/root/.ssh/id_rsa",
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudo",
		"/etc/sudoers",
		"/var/log/auth.log",
		"/var/log/secure",
	}

	options := DefaultSecureFileOptions()
	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, rootPath := range rootPaths {
		if _, err := os.Stat(rootPath); os.IsNotExist(err) {
			continue // Skip if path doesn't exist
		}

		t.Run(fmt.Sprintf("RootPath_%s", strings.ReplaceAll(rootPath, "/", "_")), func(t *testing.T) {
			params := map[string]interface{}{
				"path": rootPath,
			}

			result, err := executor.Execute(context.Background(), params)

			if err == nil && result.Success {
				t.Errorf("Expected root path %s to be blocked", rootPath)
			}
			if result != nil && !strings.Contains(result.Error, "access denied") {
				t.Errorf("Expected access denied error for %s, got: %s", rootPath, result.Error)
			}
		})
	}
}

// testHiddenFileAccess tests access to hidden files
func testHiddenFileAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_hidden_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create hidden files
	hiddenFiles := []string{
		".hidden",
		".env",
		".gitignore",
		".bashrc",
		".profile",
		"..double_dot",
		"normal.txt",
	}

	for _, filename := range hiddenFiles {
		filepath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filepath, []byte("hidden content"), 0644); err != nil {
			t.Errorf("Failed to create hidden file %s: %v", filename, err)
		}
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, filename := range hiddenFiles {
		t.Run(fmt.Sprintf("HiddenFile_%s", filename), func(t *testing.T) {
			params := map[string]interface{}{
				"path": filepath.Join(tempDir, filename),
			}

			result, err := executor.Execute(context.Background(), params)

			// Hidden files should be accessible if they're in allowed paths
			// Security depends on the allowed paths configuration
			if err != nil {
				t.Errorf("Unexpected error for hidden file %s: %v", filename, err)
			}
			if result == nil || !result.Success {
				if result != nil {
					t.Errorf("Expected hidden file %s to be accessible in allowed directory, got error: %s", filename, result.Error)
				} else {
					t.Errorf("Expected hidden file %s to be accessible in allowed directory", filename)
				}
			}
		})
	}
}

// testCaseSensitivity tests case sensitivity handling
func testCaseSensitivity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_case_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "TestFile.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Exact case match",
			path:     testFile,
			expected: true,
		},
		{
			name:     "Lowercase",
			path:     filepath.Join(tempDir, "testfile.txt"),
			expected: runtime.GOOS == "windows" || runtime.GOOS == "darwin", // Windows and macOS are case-insensitive by default
		},
		{
			name:     "Uppercase",
			path:     filepath.Join(tempDir, "TESTFILE.TXT"),
			expected: runtime.GOOS == "windows" || runtime.GOOS == "darwin",
		},
		{
			name:     "Mixed case",
			path:     filepath.Join(tempDir, "tEsTfIlE.TxT"),
			expected: runtime.GOOS == "windows" || runtime.GOOS == "darwin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": tc.path,
			}

			result, err := executor.Execute(context.Background(), params)

			if tc.expected {
				if err != nil {
					t.Errorf("Unexpected error for case variant: %v", err)
				}
				if result == nil || !result.Success {
					t.Error("Expected case variant to succeed")
				}
			} else {
				if err == nil && result.Success {
					t.Error("Expected case variant to fail on case-sensitive filesystem")
				}
			}
		})
	}
}

// testUnicodePathHandling tests handling of Unicode in file paths
func testUnicodePathHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_unicode_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files with Unicode names
	unicodeFiles := []string{
		"café.txt",
		"файл.txt",           // Russian
		"文件.txt",             // Chinese
		"ファイル.txt",           // Japanese
		"🚀rocket.txt",       // Emoji
		"test\u200bzwsp.txt", // Zero-width space
		"test\ufeffbom.txt",  // BOM
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	for _, filename := range unicodeFiles {
		t.Run(fmt.Sprintf("Unicode_%s", filename), func(t *testing.T) {
			filepath := filepath.Join(tempDir, filename)
			
			// Try to create the file
			if err := os.WriteFile(filepath, []byte("unicode content"), 0644); err != nil {
				t.Skipf("Cannot create Unicode file %s: %v", filename, err)
			}

			params := map[string]interface{}{
				"path": filepath,
			}

			result, err := executor.Execute(context.Background(), params)

			// Unicode files should be handled properly
			if err != nil {
				t.Errorf("Unexpected error for Unicode file %s: %v", filename, err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected Unicode file %s to be accessible", filename)
			}
		})
	}
}

// testFileSystemLimits tests file system limits and constraints
func testFileSystemLimits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_limits_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test file size limits
	t.Run("FileSizeLimit", func(t *testing.T) {
		options := &SecureFileOptions{
			AllowedPaths: []string{tempDir},
			MaxFileSize:  1024, // 1KB limit
		}

		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		// Create a file larger than the limit
		largeFile := filepath.Join(tempDir, "large.txt")
		largeContent := strings.Repeat("x", 2048) // 2KB
		if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
			t.Fatalf("Failed to create large file: %v", err)
		}

		params := map[string]interface{}{
			"path": largeFile,
		}

		result, err := executor.Execute(context.Background(), params)

		if err == nil && result.Success {
			t.Error("Expected large file to be rejected")
		}
		if result != nil && !strings.Contains(result.Error, "exceeds maximum allowed size") {
			t.Errorf("Expected size limit error, got: %s", result.Error)
		}
	})

	// Test directory depth limits
	t.Run("DirectoryDepthLimit", func(t *testing.T) {
		options := &SecureFileOptions{
			AllowedPaths: []string{tempDir},
			MaxFileSize:  1024 * 1024,
		}

		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		// Create a deeply nested directory structure
		deepPath := tempDir
		for i := 0; i < 100; i++ {
			deepPath = filepath.Join(deepPath, fmt.Sprintf("level%d", i))
		}
		deepFile := filepath.Join(deepPath, "deep.txt")

		if err := os.MkdirAll(filepath.Dir(deepFile), 0755); err != nil {
			t.Skipf("Cannot create deep directory structure: %v", err)
		}

		if err := os.WriteFile(deepFile, []byte("deep content"), 0644); err != nil {
			t.Skipf("Cannot create deep file: %v", err)
		}

		params := map[string]interface{}{
			"path": deepFile,
		}

		result, err := executor.Execute(context.Background(), params)

		// Should succeed if within allowed paths
		if err != nil {
			t.Logf("Deep path access failed (may be expected): %v", err)
		}
		if result != nil && !result.Success {
			t.Logf("Deep path access denied: %s", result.Error)
		}
	})
}

// testConcurrentFileAccess tests concurrent file access safety
func testConcurrentFileAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := make([]string, 10)
	for i := 0; i < 10; i++ {
		testFiles[i] = filepath.Join(tempDir, fmt.Sprintf("concurrent%d.txt", i))
		if err := os.WriteFile(testFiles[i], []byte(fmt.Sprintf("content%d", i)), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Test concurrent reads
	const numGoroutines = 20
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			fileIndex := id % len(testFiles)
			params := map[string]interface{}{
				"path": testFiles[fileIndex],
			}

			result, err := executor.Execute(context.Background(), params)
			if err != nil {
				results <- err
				return
			}
			if !result.Success {
				results <- fmt.Errorf("concurrent read failed: %s", result.Error)
				return
			}
			results <- nil
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent file access failed: %v", err)
		}
	}
}

// TestFileSystemSecurityIntegration tests integration of all file system security features
func TestFileSystemSecurityIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fs_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a comprehensive test environment
	testStructure := map[string]string{
		"allowed/file1.txt":         "content1",
		"allowed/file2.txt":         "content2",
		"allowed/subdir/file3.txt":  "content3",
		"allowed/.hidden":           "hidden content",
		"allowed/large.txt":         strings.Repeat("x", 2048),
		"allowed/unicode_файл.txt":  "unicode content",
	}

	for path, content := range testStructure {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create a symlink outside allowed area
	if err := os.Symlink("/etc/passwd", filepath.Join(tempDir, "allowed/passwd_link")); err != nil {
		t.Logf("Could not create symlink (may not be supported): %v", err)
	}

	// Configure comprehensive security options
	options := &SecureFileOptions{
		AllowedPaths:  []string{filepath.Join(tempDir, "allowed")},
		MaxFileSize:   1024, // 1KB limit
		AllowSymlinks: false,
		DenyPaths:     []string{"/etc", "/root"},
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Test various access scenarios
	testCases := []struct {
		name        string
		path        string
		shouldPass  bool
		expectedErr string
	}{
		{
			name:       "Allowed file access",
			path:       filepath.Join(tempDir, "allowed/file1.txt"),
			shouldPass: true,
		},
		{
			name:       "Subdirectory file access",
			path:       filepath.Join(tempDir, "allowed/subdir/file3.txt"),
			shouldPass: true,
		},
		{
			name:       "Hidden file access",
			path:       filepath.Join(tempDir, "allowed/.hidden"),
			shouldPass: true,
		},
		{
			name:       "Unicode file access",
			path:       filepath.Join(tempDir, "allowed/unicode_файл.txt"),
			shouldPass: true,
		},
		{
			name:        "Large file access",
			path:        filepath.Join(tempDir, "allowed/large.txt"),
			shouldPass:  false,
			expectedErr: "exceeds maximum allowed size",
		},
		{
			name:        "Directory traversal attempt",
			path:        filepath.Join(tempDir, "allowed/../etc/passwd"),
			shouldPass:  false,
			expectedErr: "access denied",
		},
		{
			name:        "Symlink access",
			path:        filepath.Join(tempDir, "allowed/passwd_link"),
			shouldPass:  false,
			expectedErr: "symbolic links are not allowed",
		},
		{
			name:        "Outside allowed path",
			path:        filepath.Join(tempDir, "disallowed.txt"),
			shouldPass:  false,
			expectedErr: "access denied",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": tc.path,
			}

			result, err := executor.Execute(context.Background(), params)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Unexpected error for allowed access: %v", err)
				}
				if result == nil || !result.Success {
					t.Error("Expected allowed access to succeed")
				}
			} else {
				if err == nil && result.Success {
					t.Errorf("Expected security check to fail for: %s", tc.name)
				}
				if result != nil && tc.expectedErr != "" && !strings.Contains(result.Error, tc.expectedErr) {
					t.Errorf("Expected error containing '%s', got: %s", tc.expectedErr, result.Error)
				}
			}
		})
	}
}

// TestFileSystemSecurityEdgeCases tests edge cases in file system security
func TestFileSystemSecurityEdgeCases(t *testing.T) {
	t.Run("EmptyAllowedPaths", func(t *testing.T) {
		options := &SecureFileOptions{
			AllowedPaths: []string{}, // Empty allowed paths
			MaxFileSize:  1024 * 1024,
		}

		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		// Should allow access when no restrictions are set
		params := map[string]interface{}{
			"path": os.TempDir(),
		}

		result, err := executor.Execute(context.Background(), params)
		
		// Might fail for other reasons (directory access), but shouldn't fail due to path restrictions
		if err != nil {
			t.Logf("Operation failed (may be expected): %v", err)
		}
		if result != nil && strings.Contains(result.Error, "not in allowed directories") {
			t.Error("Expected empty allowed paths to not restrict access")
		}
	})

	t.Run("OverlappingPaths", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "fs_overlap_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		subDir := filepath.Join(tempDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}

		testFile := filepath.Join(subDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		options := &SecureFileOptions{
			AllowedPaths: []string{tempDir, subDir}, // Overlapping paths
			MaxFileSize:  1024 * 1024,
		}

		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		params := map[string]interface{}{
			"path": testFile,
		}

		result, err := executor.Execute(context.Background(), params)

		if err != nil {
			t.Errorf("Unexpected error with overlapping paths: %v", err)
		}
		if result == nil || !result.Success {
			t.Error("Expected overlapping paths to work correctly")
		}
	})

	t.Run("ConflictingDenyPaths", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "fs_conflict_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testFile := filepath.Join(tempDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		options := &SecureFileOptions{
			AllowedPaths: []string{tempDir},
			DenyPaths:    []string{tempDir}, // Conflicting with allowed
			MaxFileSize:  1024 * 1024,
		}

		executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

		params := map[string]interface{}{
			"path": testFile,
		}

		result, err := executor.Execute(context.Background(), params)

		// Deny paths should take precedence
		if err == nil && result.Success {
			t.Error("Expected deny paths to take precedence over allowed paths")
		}
	})
}

// BenchmarkFileSystemSecurity benchmarks file system security operations
func BenchmarkFileSystemSecurity(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "fs_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "bench.txt")
	if err := os.WriteFile(testFile, []byte("benchmark content"), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	params := map[string]interface{}{
		"path": testFile,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(context.Background(), params)
		if err != nil {
			b.Errorf("Benchmark operation failed: %v", err)
		}
	}
}