# Go Patterns and Gotchas

Production Go patterns and common mistakes to avoid.

---

## Production Principles

### 1. Accept Interfaces, Return Structs

```go
// BAD - accepts concrete type
func ProcessTasks(repo *PostgresTaskRepository) error {
    // Can't test without real database
}

// GOOD - accepts interface
func ProcessTasks(repo TaskRepository) error {
    // Can pass mock in tests
}

// BAD - returns interface
func NewTaskRepository() TaskRepository {
    return &PostgresTaskRepository{}
}

// GOOD - returns concrete type
func NewPostgresTaskRepository(db *pgx.Pool) *PostgresTaskRepository {
    return &PostgresTaskRepository{db: db}
}
```

**Rule**: Define interfaces where they're *used*, not where they're *implemented*.

### 2. Small Interfaces

```go
// BAD - kitchen sink
type TaskManager interface {
    Create(...) error
    Get(...) (*Task, error)
    Update(...) error
    Delete(...) error
    List(...) ([]*Task, error)
    // ... 10 more methods
}

// GOOD - focused
type TaskReader interface {
    Get(ctx context.Context, id string) (*Task, error)
}

type TaskWriter interface {
    Save(ctx context.Context, t *Task) error
}

// Compose when needed
type TaskRepository interface {
    TaskReader
    TaskWriter
}
```

### 3. Explicit Dependencies

```go
// BAD - hidden dependency
var db *pgx.Pool // package-level global

func GetTask(id string) (*Task, error) {
    return db.Query(...) // Where did db come from?
}

// GOOD - explicit
type TaskService struct {
    repo   TaskRepository
    cache  Cache
    logger *slog.Logger
}

func NewTaskService(repo TaskRepository, cache Cache, logger *slog.Logger) *TaskService {
    return &TaskService{repo: repo, cache: cache, logger: logger}
}
```

### 4. Context is Non-Negotiable

```go
// BAD
func FetchData(url string) ([]byte, error)

// GOOD
func FetchData(ctx context.Context, url string) ([]byte, error)
```

**Rules**:
1. Never store context in a struct
2. Always pass context as first parameter
3. Don't pass nil context - use `context.Background()` or `context.TODO()`
4. Check `ctx.Done()` before expensive operations

### 5. Errors Are Values

```go
// BAD - loses context
if err != nil {
    return err
}

// GOOD - adds context
if err != nil {
    return fmt.Errorf("fetch task %s: %w", id, err)
}
```

**Sentinel errors**:
```go
var ErrNotFound = errors.New("not found")

if errors.Is(err, ErrNotFound) {
    // Handle
}
```

### 6. Zero Values Are Useful

```go
// GOOD - zero value works
var counter Counter
counter.Inc() // Works

// BAD - requires initialization
var client Client
client.Do() // Panics on nil httpClient
```

---

## Common Gotchas

### 1. Nil Interface vs Nil Pointer

```go
type Writer interface {
    Write([]byte) error
}

func getWriter() Writer {
    var w *bytes.Buffer // nil pointer
    return w            // NOT a nil interface!
}

func main() {
    w := getWriter()
    if w == nil {
        fmt.Println("nil") // Does NOT print!
    }
    w.Write([]byte("hello")) // PANIC
}
```

**Fix**: Return untyped nil:
```go
func getWriter() Writer {
    var w *bytes.Buffer
    if w == nil {
        return nil
    }
    return w
}
```

### 2. Maps Are Not Concurrent-Safe

```go
// BROKEN - will panic
m := make(map[string]int)
go func() { m["a"] = 1 }()
go func() { m["b"] = 2 }()

// FIX: Use sync.RWMutex or sync.Map
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}
```

### 3. Defer Evaluates Arguments Immediately

```go
func example() {
    i := 0
    defer fmt.Println(i) // Prints 0, not 1
    i++
}

// Use closure for final value
defer func() { fmt.Println(i) }()
```

### 4. Defer in Loops

```go
// BROKEN - opens all files before closing any
for _, path := range paths {
    f, _ := os.Open(path)
    defer f.Close() // Won't close until function returns
}

// FIX - wrap in function
for _, path := range paths {
    func() {
        f, _ := os.Open(path)
        defer f.Close()
        // ...
    }()
}
```

### 5. Slice Append Can Mutate Original

```go
original := make([]int, 3, 6)
slice1 := original[:2]
slice2 := append(slice1, 99)
// original is now [?, ?, 99] - MUTATED!

// FIX - copy to new backing array
independent := append([]int(nil), original...)
```

### 6. Channel Close Panics

```go
ch := make(chan int)
close(ch)
close(ch) // PANIC: close of closed channel
```

**Rule**: Only the sender should close. Receiver never closes.

### 7. JSON Ignores Unexported Fields

```go
type Config struct {
    Name   string `json:"name"`
    secret string `json:"secret"` // lowercase = unexported
}

// secret will NOT be set from JSON
```

### 8. Time Comparisons Need .Equal()

```go
t1 := time.Now()
t2 := t1
fmt.Println(t1 == t2)      // Might be false!
fmt.Println(t1.Equal(t2))  // Always true
```

---

## What Panics

| Operation | Panics? |
|-----------|---------|
| Nil pointer dereference | Yes |
| Out of bounds slice/array | Yes |
| Nil map write | Yes |
| Nil map read | No (returns zero) |
| Close nil channel | Yes |
| Close closed channel | Yes |
| Send on closed channel | Yes |
| Send on nil channel | No (blocks forever) |
| Receive from closed | No (returns zero) |
| Receive from nil | No (blocks forever) |
| Type assertion (single return) | Yes |
| Type assertion (two returns) | No |

---

## Patterns

### Functional Options

```go
type Option func(*Client)

func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.timeout = d }
}

func NewClient(opts ...Option) *Client {
    c := &Client{timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

### Table-Driven Tests

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Result
        wantErr bool
    }{
        {"valid", "good", Result{}, false},
        {"empty", "", Result{}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Graceful Shutdown

```go
func main() {
    server := &http.Server{Addr: ":8080", Handler: handler}

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    <-stop

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    server.Shutdown(ctx)
}
```

### Structured Logging

```go
logger := slog.With("component", "flowgraph", "run_id", runID)

logger.Info("node started", "node_id", nodeID)
logger.Error("node failed", "node_id", nodeID, "error", err)
```

---

## Performance Tips

### Preallocate Slices

```go
// BAD - grows multiple times
var result []string
for _, item := range items {
    result = append(result, item.Name)
}

// GOOD - single allocation
result := make([]string, 0, len(items))
for _, item := range items {
    result = append(result, item.Name)
}
```

### Use strings.Builder

```go
// BAD - allocates on every +
result := ""
for _, s := range strings {
    result += s
}

// GOOD
var b strings.Builder
for _, s := range strings {
    b.WriteString(s)
}
result := b.String()
```

### sync.Pool for Frequent Allocations

```go
var bufferPool = sync.Pool{
    New: func() interface{} { return new(bytes.Buffer) },
}

func Process(data []byte) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    // Use buf
}
```

---

## Linting Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - gosec          # Security
    - bodyclose      # HTTP body not closed
    - nilerr         # Returns nil with non-nil err
    - errcheck       # Unchecked errors
    - errorlint      # Error wrapping
    - gofmt
    - goimports
    - revive
    - gocyclo
    - funlen

linters-settings:
  gocyclo:
    min-complexity: 15
  funlen:
    lines: 100
    statements: 50
```

---

## Golden Rules

1. Always check `ok` from type assertions: `v, ok := x.(T)`
2. Always check `ok` from map access: `v, ok := m[k]`
3. Never close channels from receiver side
4. When in doubt, copy
5. Run tests with `-race`
6. Don't ignore errors, ever
7. Accept interfaces, return structs
8. Context as first parameter
9. Wrap errors with context
10. Make zero values useful
