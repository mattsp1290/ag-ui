# Type Safety Best Practices

## Overview

This guide establishes coding standards and best practices for type-safe development in the AG-UI Go SDK. Following these practices ensures consistent, maintainable, and performant code while maximizing the benefits of Go's type system.

## Table of Contents

1. [Coding Standards](#coding-standards)
2. [Generic Usage Patterns](#generic-usage-patterns)
3. [Performance Considerations](#performance-considerations)
4. [Testing Patterns](#testing-patterns)
5. [IDE Setup and Productivity](#ide-setup-and-productivity)
6. [Error Handling](#error-handling)
7. [Documentation Standards](#documentation-standards)
8. [Code Review Guidelines](#code-review-guidelines)

---

## Coding Standards

### Type Definition Guidelines

#### 1. Prefer Explicit Types Over interface{}

**❌ Avoid:**
```go
type Config struct {
    Settings map[string]interface{} `json:"settings"`
}

func ProcessData(data interface{}) error {
    // Type assertion required
}
```

**✅ Prefer:**
```go
type Config struct {
    Settings ConfigSettings `json:"settings"`
}

type ConfigSettings struct {
    DatabaseURL    string `json:"database_url"`
    Port          int    `json:"port"`
    EnableLogging bool   `json:"enable_logging"`
}

func ProcessData[T Processable](data T) error {
    // Type-safe processing
}
```

#### 2. Use Meaningful Type Names

**❌ Avoid:**
```go
type Data struct { ... }
type Info struct { ... }
type Item struct { ... }
```

**✅ Prefer:**
```go
type UserData struct { ... }
type SystemInfo struct { ... }
type ConfigItem struct { ... }
```

#### 3. Group Related Types

```go
// User-related types
type User struct {
    ID       string    `json:"id"`
    Name     string    `json:"name"`
    Email    string    `json:"email"`
    Created  time.Time `json:"created"`
}

type UserPreferences struct {
    Theme      string `json:"theme"`
    Language   string `json:"language"`
    Timezone   string `json:"timezone"`
}

type UserSession struct {
    UserID    string    `json:"user_id"`
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
}
```

### Struct Design Best Practices

#### 1. Use Validation Tags

```go
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=2,max=50"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"min=18,max=120"`
    Role     string `json:"role" validate:"oneof=user admin moderator"`
}
```

#### 2. Implement String() Method for Important Types

```go
type EventType string

const (
    EventTypeUserLogin  EventType = "user_login"
    EventTypeUserLogout EventType = "user_logout"
)

func (et EventType) String() string {
    return string(et)
}

func (et EventType) IsValid() bool {
    switch et {
    case EventTypeUserLogin, EventTypeUserLogout:
        return true
    default:
        return false
    }
}
```

#### 3. Provide Zero Value Safety

```go
type SafeCounter struct {
    mu    sync.RWMutex
    value int64
}

// NewSafeCounter creates a properly initialized counter
func NewSafeCounter() *SafeCounter {
    return &SafeCounter{}
}

// Zero value is safe to use
func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}
```

---

## Generic Usage Patterns

### When to Use Generics

#### 1. Collection Operations

**Good Use Cases:**
```go
// Generic collection utilities
func Map[T, R any](slice []T, fn func(T) R) []R {
    result := make([]R, len(slice))
    for i, v := range slice {
        result[i] = fn(v)
    }
    return result
}

func Filter[T any](slice []T, predicate func(T) bool) []T {
    var result []T
    for _, v := range slice {
        if predicate(v) {
            result = append(result, v)
        }
    }
    return result
}

// Usage
users := []User{ /* ... */ }
activeUsers := Filter(users, func(u User) bool {
    return u.Active
})
```

#### 2. Type-Safe Containers

```go
// Generic result type for error handling
type Result[T any] struct {
    Value T
    Error error
}

func NewResult[T any](value T, err error) Result[T] {
    return Result[T]{Value: value, Error: err}
}

func (r Result[T]) IsOK() bool {
    return r.Error == nil
}

func (r Result[T]) Unwrap() (T, error) {
    return r.Value, r.Error
}

// Generic optional type
type Optional[T any] struct {
    value   T
    present bool
}

func Some[T any](value T) Optional[T] {
    return Optional[T]{value: value, present: true}
}

func None[T any]() Optional[T] {
    return Optional[T]{present: false}
}

func (o Optional[T]) IsPresent() bool {
    return o.present
}

func (o Optional[T]) Get() (T, bool) {
    return o.value, o.present
}
```

#### 3. Interface Constraints

```go
// Define useful constraints
type Comparable interface {
    comparable
}

type Ordered interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
    ~float32 | ~float64 |
    ~string
}

type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
    ~float32 | ~float64
}

// Generic functions with constraints
func Max[T Ordered](a, b T) T {
    if a > b {
        return a
    }
    return b
}

func Sum[T Numeric](values []T) T {
    var sum T
    for _, v := range values {
        sum += v
    }
    return sum
}
```

### When NOT to Use Generics

#### 1. Simple Functions

**❌ Over-engineered:**
```go
func Add[T ~int | ~float64](a, b T) T {
    return a + b
}
```

**✅ Keep it simple:**
```go
func AddInt(a, b int) int {
    return a + b
}

func AddFloat(a, b float64) float64 {
    return a + b
}
```

#### 2. Type Lists Too Long

**❌ Avoid:**
```go
type ManyTypes interface {
    int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64 | string | bool | []byte | time.Time | /* ... many more */
}
```

**✅ Prefer specific interfaces:**
```go
type Serializable interface {
    MarshalJSON() ([]byte, error)
    UnmarshalJSON([]byte) error
}
```

---

## Performance Considerations

### Memory Optimization

#### 1. Struct Field Ordering

```go
// ❌ Poor alignment (24 bytes on 64-bit)
type BadStruct struct {
    flag1 bool    // 1 byte + 7 padding
    count int64   // 8 bytes
    flag2 bool    // 1 byte + 7 padding
}

// ✅ Good alignment (16 bytes on 64-bit)
type GoodStruct struct {
    count int64   // 8 bytes
    flag1 bool    // 1 byte
    flag2 bool    // 1 byte + 6 padding
}
```

#### 2. Pool Reusable Objects

```go
// Pool expensive-to-create objects
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 1024)
    },
}

func ProcessLargeData(data []byte) error {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf[:0])
    
    // Use buf for processing
    return nil
}
```

#### 3. Prefer Value Types When Appropriate

```go
// For small, immutable data
type Point struct {
    X, Y float64
}

func Distance(p1, p2 Point) float64 {
    dx := p1.X - p2.X
    dy := p1.Y - p2.Y
    return math.Sqrt(dx*dx + dy*dy)
}

// For large or mutable data, use pointers
type LargeStruct struct {
    data [1000]int
}

func ProcessLarge(ls *LargeStruct) {
    // Process without copying
}
```

### CPU Optimization

#### 1. Avoid Unnecessary Boxing

**❌ Avoid:**
```go
func ProcessValues(values []interface{}) {
    for _, v := range values {
        // Boxing/unboxing overhead
        if str, ok := v.(string); ok {
            processString(str)
        }
    }
}
```

**✅ Prefer:**
```go
func ProcessStrings(values []string) {
    for _, v := range values {
        processString(v) // No boxing
    }
}

func ProcessValues[T any](values []T, processor func(T)) {
    for _, v := range values {
        processor(v) // Type-safe, no boxing
    }
}
```

#### 2. Use Efficient Slice Operations

```go
// Efficient slice append with capacity hint
func AppendEfficiently[T any](base []T, items ...T) []T {
    if cap(base)-len(base) >= len(items) {
        return append(base, items...)
    }
    
    // Grow with better capacity
    newCap := len(base) + len(items)
    if newCap < 2*cap(base) {
        newCap = 2 * cap(base)
    }
    
    newSlice := make([]T, len(base), newCap)
    copy(newSlice, base)
    return append(newSlice, items...)
}
```

#### 3. Optimize Hot Paths

```go
// Fast path for common cases
func ParseUserID(input string) (int64, error) {
    // Fast path for simple numbers
    if len(input) > 0 && input[0] >= '0' && input[0] <= '9' {
        if id, err := strconv.ParseInt(input, 10, 64); err == nil {
            return id, nil
        }
    }
    
    // Slow path for complex parsing
    return parseComplexUserID(input)
}
```

---

## Testing Patterns

### Type-Safe Test Data

#### 1. Use Builder Pattern for Test Data

```go
type UserBuilder struct {
    user User
}

func NewUserBuilder() *UserBuilder {
    return &UserBuilder{
        user: User{
            ID:      generateID(),
            Name:    "Test User",
            Email:   "test@example.com",
            Active:  true,
            Created: time.Now(),
        },
    }
}

func (b *UserBuilder) WithName(name string) *UserBuilder {
    b.user.Name = name
    return b
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
    b.user.Email = email
    return b
}

func (b *UserBuilder) Inactive() *UserBuilder {
    b.user.Active = false
    return b
}

func (b *UserBuilder) Build() User {
    return b.user
}

// Usage in tests
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name string
        user User
        want error
    }{
        {
            name: "valid user",
            user: NewUserBuilder().Build(),
            want: nil,
        },
        {
            name: "invalid email",
            user: NewUserBuilder().WithEmail("invalid").Build(),
            want: ErrInvalidEmail,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ValidateUser(tt.user)
            if got != tt.want {
                t.Errorf("ValidateUser() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### 2. Generic Test Helpers

```go
// Generic assertion helpers
func AssertEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}

func AssertNotEqual[T comparable](t *testing.T, got, notWant T) {
    t.Helper()
    if got == notWant {
        t.Errorf("got %v, but didn't want it", got)
    }
}

func AssertContains[T comparable](t *testing.T, slice []T, item T) {
    t.Helper()
    for _, v := range slice {
        if v == item {
            return
        }
    }
    t.Errorf("slice %v does not contain %v", slice, item)
}

// Usage
func TestSliceOperations(t *testing.T) {
    nums := []int{1, 2, 3, 4, 5}
    filtered := Filter(nums, func(n int) bool { return n%2 == 0 })
    
    AssertEqual(t, len(filtered), 2)
    AssertContains(t, filtered, 2)
    AssertContains(t, filtered, 4)
}
```

#### 3. Mock Interfaces with Generics

```go
// Generic mock for typed interfaces
type MockRepository[T any] struct {
    items map[string]T
    calls []string
}

func NewMockRepository[T any]() *MockRepository[T] {
    return &MockRepository[T]{
        items: make(map[string]T),
        calls: make([]string, 0),
    }
}

func (m *MockRepository[T]) Get(id string) (T, error) {
    m.calls = append(m.calls, fmt.Sprintf("Get(%s)", id))
    item, exists := m.items[id]
    if !exists {
        var zero T
        return zero, fmt.Errorf("not found")
    }
    return item, nil
}

func (m *MockRepository[T]) Save(id string, item T) error {
    m.calls = append(m.calls, fmt.Sprintf("Save(%s)", id))
    m.items[id] = item
    return nil
}

func (m *MockRepository[T]) AssertCalled(t *testing.T, method string) {
    for _, call := range m.calls {
        if strings.Contains(call, method) {
            return
        }
    }
    t.Errorf("expected method %s to be called", method)
}

// Usage
func TestUserService(t *testing.T) {
    mockRepo := NewMockRepository[User]()
    service := NewUserService(mockRepo)
    
    user := NewUserBuilder().Build()
    err := service.CreateUser(user)
    
    AssertEqual(t, err, nil)
    mockRepo.AssertCalled(t, "Save")
}
```

### Property-Based Testing

```go
import "pgregory.net/rapid"

func TestUserEmailValidation(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate valid email
        localPart := rapid.StringMatching(`[a-zA-Z0-9]+`).Draw(t, "local")
        domain := rapid.StringMatching(`[a-zA-Z0-9]+\.[a-z]{2,}`).Draw(t, "domain")
        email := localPart + "@" + domain
        
        user := NewUserBuilder().WithEmail(email).Build()
        err := ValidateUser(user)
        
        if err != nil {
            t.Errorf("valid email %s should not fail validation: %v", email, err)
        }
    })
}

func TestSliceInvariants(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        slice := rapid.SliceOf(rapid.Int()).Draw(t, "slice")
        
        // Property: filter then length <= original length
        filtered := Filter(slice, func(n int) bool { return n > 0 })
        if len(filtered) > len(slice) {
            t.Errorf("filtered slice longer than original")
        }
        
        // Property: map preserves length
        mapped := Map(slice, func(n int) int { return n * 2 })
        if len(mapped) != len(slice) {
            t.Errorf("mapped slice has different length")
        }
    })
}
```

---

## IDE Setup and Productivity

### VS Code Configuration

#### 1. Settings for Type Safety

```json
{
    "go.useLanguageServer": true,
    "go.languageServerFlags": [
        "-rpc.trace",
        "serve",
        "--debug=localhost:6060"
    ],
    "go.lintTool": "golangci-lint",
    "go.lintFlags": [
        "--enable-all",
        "--disable=gochecknoglobals,gochecknoinits"
    ],
    "go.vetFlags": [
        "-composites=false",
        "-unusedresult"
    ],
    "go.buildFlags": [
        "-race"
    ],
    "go.testFlags": [
        "-race",
        "-cover"
    ],
    "editor.codeActionsOnSave": {
        "source.organizeImports": true,
        "source.fixAll": true
    },
    "go.generateTestsFlags": [
        "-template=testify"
    ]
}
```

#### 2. Code Snippets

```json
{
    "Generic Function": {
        "prefix": "genfunc",
        "body": [
            "func ${1:FunctionName}[${2:T} ${3:any}](${4:param} ${2:T}) ${5:T} {",
            "\t${6:// implementation}",
            "\treturn ${7:result}",
            "}"
        ],
        "description": "Type-safe generic function"
    },
    "Builder Pattern": {
        "prefix": "builder",
        "body": [
            "type ${1:Type}Builder struct {",
            "\t${2:field} ${1:Type}",
            "}",
            "",
            "func New${1:Type}Builder() *${1:Type}Builder {",
            "\treturn &${1:Type}Builder{",
            "\t\t${2:field}: ${1:Type}{${3:// defaults}},",
            "\t}",
            "}",
            "",
            "func (b *${1:Type}Builder) With${4:Field}(${5:value} ${6:type}) *${1:Type}Builder {",
            "\tb.${2:field}.${4:Field} = ${5:value}",
            "\treturn b",
            "}",
            "",
            "func (b *${1:Type}Builder) Build() ${1:Type} {",
            "\treturn b.${2:field}",
            "}"
        ],
        "description": "Builder pattern for type-safe construction"
    }
}
```

### GoLand/IntelliJ Configuration

#### 1. Live Templates

```xml
<template name="gentype" value="type $NAME$[$CONSTRAINT$ $TYPE$] struct {&#10;    $FIELD$ $FIELDTYPE$&#10;}" description="Generic type definition" toReformat="true" toShortenFQNames="true">
  <variable name="NAME" expression="" defaultValue="MyType" alwaysStopAt="true" />
  <variable name="CONSTRAINT" expression="" defaultValue="T" alwaysStopAt="true" />
  <variable name="TYPE" expression="" defaultValue="any" alwaysStopAt="true" />
  <variable name="FIELD" expression="" defaultValue="field" alwaysStopAt="true" />
  <variable name="FIELDTYPE" expression="" defaultValue="string" alwaysStopAt="true" />
  <context>
    <option name="GO" value="true" />
  </context>
</template>
```

#### 2. Inspection Profiles

Custom inspections for type safety:
- Warn on `interface{}` usage outside allowlisted packages
- Require validation tags on public struct fields
- Enforce error handling patterns
- Detect missing nil checks

---

## Error Handling

### Type-Safe Error Patterns

#### 1. Typed Errors

```go
// Define error types
type ValidationError struct {
    Field   string `json:"field"`
    Value   string `json:"value"`
    Message string `json:"message"`
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}

type NotFoundError struct {
    Resource string `json:"resource"`
    ID       string `json:"id"`
}

func (e NotFoundError) Error() string {
    return fmt.Sprintf("%s with id %s not found", e.Resource, e.ID)
}

// Error checking with type safety
func HandleError(err error) {
    switch e := err.(type) {
    case ValidationError:
        log.Printf("Validation error on field %s: %s", e.Field, e.Message)
    case NotFoundError:
        log.Printf("Resource not found: %s/%s", e.Resource, e.ID)
    default:
        log.Printf("Unknown error: %v", err)
    }
}
```

#### 2. Result Type Pattern

```go
type Result[T any] struct {
    value T
    err   error
}

func Ok[T any](value T) Result[T] {
    return Result[T]{value: value}
}

func Err[T any](err error) Result[T] {
    return Result[T]{err: err}
}

func (r Result[T]) IsOk() bool {
    return r.err == nil
}

func (r Result[T]) IsErr() bool {
    return r.err != nil
}

func (r Result[T]) Unwrap() (T, error) {
    return r.value, r.err
}

func (r Result[T]) Expect(msg string) T {
    if r.err != nil {
        panic(fmt.Sprintf("%s: %v", msg, r.err))
    }
    return r.value
}

// Usage
func GetUser(id string) Result[User] {
    user, err := repository.FindByID(id)
    if err != nil {
        return Err[User](err)
    }
    return Ok(user)
}

func ProcessUser(id string) error {
    result := GetUser(id)
    if result.IsErr() {
        return result.err
    }
    
    user := result.value
    return processUser(user)
}
```

#### 3. Error Wrapping

```go
import "fmt"

func WrapError[T any](operation string, fn func() (T, error)) (T, error) {
    result, err := fn()
    if err != nil {
        var zero T
        return zero, fmt.Errorf("%s failed: %w", operation, err)
    }
    return result, nil
}

// Usage
func SaveUser(user User) error {
    _, err := WrapError("user validation", func() (struct{}, error) {
        return struct{}{}, ValidateUser(user)
    })
    if err != nil {
        return err
    }
    
    _, err = WrapError("database save", func() (struct{}, error) {
        return struct{}{}, repository.Save(user)
    })
    return err
}
```

---

## Documentation Standards

### Type Documentation

#### 1. Document Generic Constraints

```go
// Ordered represents types that can be compared using <, <=, >=, >
// This includes all integer types, floating-point types, and strings.
type Ordered interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
    ~float32 | ~float64 |
    ~string
}

// Sort sorts a slice of ordered elements in ascending order.
// The slice is modified in place.
//
// Example:
//   nums := []int{3, 1, 4, 1, 5}
//   Sort(nums)
//   // nums is now [1, 1, 3, 4, 5]
func Sort[T Ordered](slice []T) {
    // implementation
}
```

#### 2. Document Type Invariants

```go
// UserID represents a unique user identifier.
// Invariants:
//   - Must be non-empty
//   - Must be alphanumeric (no special characters)
//   - Length must be between 3 and 50 characters
type UserID string

func (uid UserID) Validate() error {
    if len(uid) < 3 || len(uid) > 50 {
        return fmt.Errorf("user ID length must be between 3 and 50 characters")
    }
    
    for _, r := range uid {
        if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
            return fmt.Errorf("user ID must be alphanumeric")
        }
    }
    
    return nil
}
```

#### 3. Provide Usage Examples

```go
// EventProcessor processes events of type T with validation and error handling.
//
// Type parameter T must implement the Event interface and be JSON serializable.
//
// Example usage:
//   processor := NewEventProcessor[UserEvent](validateUserEvent)
//   err := processor.Process(UserEvent{
//       UserID: "user123",
//       Action: "login",
//   })
//   if err != nil {
//       log.Printf("Processing failed: %v", err)
//   }
type EventProcessor[T Event] struct {
    validator func(T) error
}
```

---

## Code Review Guidelines

### Type Safety Checklist

#### Before Submitting PR

- [ ] No new `interface{}` usage without justification
- [ ] All public APIs use type-safe alternatives
- [ ] Validation tags added to struct fields
- [ ] Error types are well-defined
- [ ] Generic constraints are properly documented
- [ ] Performance impact considered
- [ ] Tests cover type safety aspects

#### During Code Review

#### 1. Look for Type Safety Issues

```go
// ❌ Flag this in review
func ProcessData(data interface{}) error {
    // Type assertion without checking
    str := data.(string)
    return process(str)
}

// ✅ Suggest this alternative
func ProcessData[T Processable](data T) error {
    return data.Process()
}
```

#### 2. Check Generic Usage

```go
// ❌ Over-complicated generics
func SimpleAdd[T ~int | ~int8 | ~int16 | ~int32 | ~int64](a, b T) T {
    return a + b
}

// ✅ Keep it simple
func AddInt(a, b int) int {
    return a + b
}
```

#### 3. Validate Error Handling

```go
// ❌ Silent type assertion failure
if str, ok := value.(string); ok {
    // Process but ignore if not string
}

// ✅ Explicit error handling
str, ok := value.(string)
if !ok {
    return fmt.Errorf("expected string, got %T", value)
}
```

### Review Questions

1. **Is this the simplest solution?** Avoid over-engineering with generics
2. **Are types well-named and documented?** Clear intent and usage
3. **Is error handling comprehensive?** All edge cases covered
4. **Will this perform well?** Consider memory and CPU impact
5. **Is it testable?** Can we write good tests for this?
6. **Is it maintainable?** Will future developers understand this?

---

## Summary

Following these best practices ensures:

1. **Consistency** across the codebase
2. **Performance** through efficient type usage
3. **Maintainability** through clear type definitions
4. **Testability** through type-safe test patterns
5. **Productivity** through proper tooling setup
6. **Quality** through comprehensive code review

### Quick Reference

**Do:**
- Use explicit types over `interface{}`
- Document generic constraints and invariants
- Validate inputs at API boundaries
- Use builder patterns for complex construction
- Write type-safe tests
- Handle errors explicitly

**Don't:**
- Over-use generics for simple cases
- Create overly complex type hierarchies
- Ignore validation and error handling
- Skip documentation for complex types
- Use `interface{}` without strong justification

For implementation examples, see [API Migration Reference](API_MIGRATION_REFERENCE.md).
For migration strategies, see [Type Safety Migration Guide](TYPE_SAFETY_MIGRATION.md).