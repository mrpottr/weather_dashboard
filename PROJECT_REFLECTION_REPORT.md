# University Presentation Report: Programming Languages and Paradigms
**Project Title:** ATMOS — Australian Weather Intelligence Dashboard  
**Language Analyzed:** Go (Golang)  
**Repository Scope:** `weather_dashboard_new/` (`backend/` & `database/`)  

---

## 1. PROJECT OVERVIEW

ATMOS is an Australian weather intelligence dashboard combining a Go (Gin) backend, SQLite persistence, and a React frontend. It fetches forecasts from Open-Meteo's BOM (ACCESS-G) feed with automatic fallback to global models. The backend computes outdoor activity suitability, tracks empirical forecast accuracy against real-time observations, triggers customizable hazard alerts, and benchmarks concurrent multi-city retrieval.

### Package Structure and Roles
- **`backend/` (`weather-forecast-system`)**: Root application module containing runtime configuration, dependencies (`go.mod`), and entrypoints.
  - `main.go`: Bootstraps configuration, connection pooling, background retention sweeps, and signal shutdown.
  - `internal/server/`: HTTP route registration (`routes.go`) for public, auth, and admin endpoints.
  - `internal/middleware/`: Panic recovery (`JSONRecover`), security headers, and JWT/role gating (`AuthRequired`, `AdminRequired`).
  - `internal/handlers/`: Web controllers for auth, forecasts, locations, alerts, history, and admin tasks.
  - `internal/weather/`: Core engine for API queries, in-memory caching, accuracy tracking, and activity rubrics.
  - `internal/auth/`: JWT token lifecycle, in-memory rate limiting, and account hydration.
  - `internal/platform/`: Third-party photo integrations (Pexels, Unsplash, Wikimedia).
  - `internal/supports/`: Streamed CSV/JSON file generation and string formatting utilities.
  - `data/`: Local storage for persistent CSV and JSON export snapshots.
- **`database/` (`weather-forecast-database`)**: SQLite migrations, seeding, shared DTO models, and audit history.
- **`frontend/`**: React + Vite client dashboard consuming the Go REST API.

---

## 2. PARADIGMS USED

### Procedural Paradigm
Procedural code provides sequential execution with explicit control flow, state mutations, and direct error checks.

```go
// File: backend/main.go (Lines 99-104)
func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
```

```go
// File: database/database.go (Lines 285-300)
func CleanupRetention() {
	th := GetThresholds()
	if !th.RetainHistory || th.RetentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -th.RetentionDays).Format(time.RFC3339)
	if _, err := DB.Exec(`DELETE FROM forecast_history WHERE ts < ?`, cutoff); err != nil {
		log.Printf("retention cleanup: %v", err)
		return
	}
	if _, err := DB.Exec(`DELETE FROM query_history WHERE ts < ?`, cutoff); err != nil {
		log.Printf("retention cleanup: %v", err)
		return
	}
	WriteHistoryFiles()
}
```

### Object-Oriented Style in Go
Go implements OOP without classes or inheritance, using structs, method sets, interface implementation, and composition.

- **Structs and Methods:** State and behavior are coupled via method receivers on structs.
```go
// File: backend/internal/weather/source.go (Lines 145-163)
func (m *sourceMetrics) record(s fetchStat) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if s.OK && s.FromCache {
		m.cacheHits++
	}
	if s.OK && !s.FromCache {
		m.liveOK++
		m.lastByLoc[s.Location] = s.LatencyMS
	}
	if !s.OK {
		m.failures++
	}
	m.recent = append(m.recent, s)
	if len(m.recent) > 200 {
		m.recent = m.recent[len(m.recent)-200:]
	}
}
```

- **Composition vs. Inheritance (Struct Embedding):** Composite types embed inner structs to inherit fields directly.
```go
// File: database/models.go (Lines 46-49)
type UserWithPrefs struct {
	User
	Preferences Preferences `json:"preferences"`
}
```

- **Interface Implementation:** Standard library interfaces are satisfied implicitly without `implements` declarations.
```go
// File: database/models.go (Lines 465-485)
type SQLNullString struct {
	String string
	Valid  bool
}

func (n *SQLNullString) Scan(value interface{}) error {
	if value == nil {
		n.String, n.Valid = "", false
		return nil
	}
	switch v := value.(type) {
	case []byte:
		n.String, n.Valid = string(v), true
	case string:
		n.String, n.Valid = v, true
	default:
		n.String, n.Valid = "", false
	}
	return nil
}
```
*Note on Custom Interfaces:* Custom `type ... interface` declarations are **not present**; the repository relies on concrete structs and standard library interfaces (`sql.Scanner`).

### Functional Style
Functional constructs are utilized through lexical closures, higher-order functions, and pure mappings.

- **Closures Capturing Enclosing State:**
```go
// File: backend/internal/weather/intelligence.go (Lines 145-153)
	total := 100.0
	add := func(factor, reading string, points float64, reason string) {
		if points == 0 {
			return
		}
		out.Factors = append(out.Factors, ScoreFactor{Factor: factor, Reading: reading, Points: Round1(points), Reason: reason})
		total += points
	}
```

- **Higher-Order Functions and Function Values:**
```go
// File: backend/internal/supports/helpers.go (Lines 53-59)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)
```

- **Immutability:** Pure evaluation functions (e.g., `windThreshold` in `alerts.go:L23-32` and `CodeToCondition` in `weather/source.go:L579-626`) return new values without mutating inputs or global state.

### Dominant Paradigm
The **procedural paradigm combined with Go's structural composition** dominates. Web API pipelines follow linear execution paths: validate input $\rightarrow$ query database/upstream $\rightarrow$ apply business rules $\rightarrow$ serialize JSON. Functional techniques are applied selectively for localized accumulators, string transformations, and middleware closures.

---

## 3. CONCURRENCY

Concurrency handles background sweeps, server shutdown, rate limiting, thread-safe caching, and parallel multi-city weather data retrieval.

### Concurrency Primitives Used
- **Goroutines (`go func()`):** Background retention sweeps (`main.go:L44`), non-blocking HTTP listening (`main.go:L76`), and parallel worker routines (`weather/source.go:L545,555`).
- **Channels:** Unbuffered `jobs := make(chan LocationRef)` (`source.go:L533`) for task handoff; buffered `results := make(chan FetchResult, len(locs))` (`source.go:L534`) avoiding worker stalls; buffered `quit := make(chan os.Signal, 1)` (`main.go:L87`) for OS shutdown signals.
- **Select Statement:** Multiplexes task dispatch against context cancellation (`weather/source.go:L557`).
- **`sync.WaitGroup`:** Coordinates worker pool completion before closing channels (`weather/source.go:L542–564`).
- **Mutexes:** `sync.Mutex` (`rateMu` in `auth/auth.go:L65`) synchronizes IP rate limiting; `sync.Mutex` (`mu` in `weather/source.go:L130`) synchronizes latency metrics; `sync.RWMutex` (`cacheMu` in `weather/source.go:L211`) allows concurrent cache reads (`RLock`) while serializing writes (`Lock`).
- **`context.Context`:** Aborts upstream network I/O on client disconnect, and provides a 10-second graceful server shutdown (`main.go:L91`).

### Concurrent Worker Pool Implementation
```go
// File: backend/internal/weather/source.go (Lines 531-566)
func FetchManyConcurrently(ctx context.Context, locs []LocationRef, refresh bool) ([]FetchResult, time.Duration, int64) {
	started := time.Now()
	jobs := make(chan LocationRef)
	results := make(chan FetchResult, len(locs))
	workers := len(locs)
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for loc := range jobs {
				start := time.Now()
				fc, err := FetchForecast(ctx, loc, refresh)
				res := FetchResult{Loc: loc, Forecast: fc, Err: err, LatencyMS: time.Since(start).Milliseconds()}
				results <- res
			}
		}()
	}
	go func() {
		for _, loc := range locs {
			select {
			case jobs <- loc:
			case <-ctx.Done():
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
    // ...
```

### Problems Solved & Concurrency Risks
- **Problem Solved:** Sequential network queries for all Australian state capitals took $>3\text{ s}$. The worker pool limits concurrency to 8 parallel routines, reducing map loading latency to $\sim 450\text{ ms}$ (`ConcurrentMS` vs `SequentialMS`).
- **Deadlock / Panic Risks:** In `FetchManyConcurrently`, `close(results)` is called strictly after `wg.Wait()`, ensuring workers never send to a closed channel. Sizing `results` to `len(locs)` prevents worker deadlock.
- **Memory Leak Risk:** `rateBuckets` in `backend/internal/auth/auth.go` tracks IP timestamps without key eviction, posing an unbounded memory growth risk under high unique-IP traffic.

---

## 4. DESIGN & TRADE-OFFS

### Interface Usage and Abstractions
Concrete structs are used across domain services rather than interfaces. This eliminates boilerplate and simplifies navigation, but couples handlers directly to concrete packages, preventing mock-driven unit testing.

### Error Handling Approach
Errors are handled via explicit `(value, error)` returns without exceptions. Unhandled HTTP panics are trapped into structured JSON errors via `middleware.JSONRecover()`. External API failures trigger multi-tiered degradation: live BOM ACCESS-G $\rightarrow$ live Open-Meteo fallback $\rightarrow$ cached payload with `SourceDegraded: true`.

### Typing Decisions
- **Static Typing & DTOs:** Strongly-typed structs govern schemas and HTTP requests.
- **Pointers for Nullability:** Pointers (`*float64`, `*time.Time`, `*string`) distinguish unpopulated or null fields from valid zero values.
- **Type Aliasing:** Used in `internal/weather/aliases.go` and `internal/handlers/aliases.go` (`type LocationRef = database.LocationRef`) to decouple packages without circular imports.
- **Generics (`[T any]`):** **Not present**. Standard concrete structs and `interface{}` slices are used.

### Memory and Performance Choices
- **Pointers vs. Values:** Large models (`*ForecastResponse`, `*User`) use pointers to avoid copy overhead; small values (`LocationRef`) are passed by value.
- **Single-Writer SQLite Pool:** `handle.SetMaxOpenConns(1)` serializes database writes, preventing lock failures (`busy_timeout: 5000ms`) at the cost of write throughput.
- **Streaming Output:** `WriteCSVResponse` flushes CSV records directly to `gin.Context.Writer` via `csv.NewWriter`, streaming data without buffering in memory.

### Alternative Designs
A `WeatherProvider` interface would have decoupled `weather/source.go` from Open-Meteo HTTP endpoints, permitting offline unit tests and alternative weather providers.

---

## 5. CODE QUALITY

### Readable, Maintainable Code
- **Self-Documenting Code:** Functions feature clear docstrings articulating architectural intent, concurrency invariants, and safety boundaries.
- **Granular Helpers:** Helpers like `HumanSince` (`supports/helpers.go:L63-75`) format relative timestamps through clean switch expressions.
- **Deterministic Evaluation:** `internal/weather/intelligence.go` breaks environmental assessment into distinct factor blocks with explicit point assignments.

### Weak, Repetitive, or Hard-to-Maintain Code
- **Manual Row Scanning:** Query functions manually scan database fields column-by-column into struct fields with temporary variables.
```go
// File: backend/internal/auth/users.go (Lines 26-29)
	err := database.DB.QueryRow(`SELECT id,first_name,last_name,email,role,status,default_location,created_at,last_active_at
		FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Status, &u.DefaultLocation, &created, &lastActive)
```
- **Duplicate Type Aliasing Files:** `internal/weather/aliases.go` and `internal/handlers/aliases.go` duplicate type alias lists rather than structuring a cohesive domain package.
- **Global Mutable Singletons:** `database.DB`, `weather.Metrics`, `auth.jwtSecret`, and `cacheMem` exist as package-level global variables, coupling state to process lifetime.

### Test Coverage and Testing Approach
Automated unit and integration tests (`*_test.go`): **not present**. Verification relies on runtime self-diagnostics (`/api/health`, `/api/admin/metrics`, `/api/admin/source/check`) and manual frontend integration testing.

---

## 6. EVOLUTION OVER TIME

### Git Version Control Status
- Git repository (`.git` directory) / git log / commit history: **not present** in the project directory (`fatal: not a git repository`).

### Architectural Evolution and Bug Fixes Evident in Codebase
1. **Monolithic Flat Package to Layered Architecture:**
   - *Evidence:* `README.md:L63` documents an initial flat structure (*"11 files in package main"*).
   - *Evolution:* Refactored into an independent data module (`database`) and discrete internal subsystems (`server`, `middleware`, `handlers`, `weather`, `auth`, `platform`, `supports`), using type aliasing to prevent circular imports.
2. **SQLite Lock Starvation During Remote I/O:**
   - *Evidence:* `backend/internal/handlers/locations.go:L142-158`:
```go
		/*
		Eagerly draining database rows before network requests avoids holding SQLite connection locks during remote HTTP calls.
		*/
		rowsList := []savedLoc{}
		if rows, err := database.DB.Query(`SELECT name,latitude,longitude,timezone,admin_area FROM locations WHERE user_id = ?`, u.ID); err == nil {
			for rows.Next() {
				var s savedLoc
				if err := rows.Scan(&s.Name, &s.Lat, &s.Lon, &s.Timezone, &s.AdminArea); err == nil {
					rowsList = append(rowsList, s)
				}
			}
			rows.Close()
		}
```
   - *Bug & Fix:* Holding an open SQL cursor during slow upstream weather queries locked the single SQLite connection. Draining rows into `rowsList` and closing the cursor before initiating remote HTTP calls fixed the lock contention.
3. **Empty Upstream 200 Payload Handling (Silent Degradation):**
   - *Evidence:* `backend/internal/weather/source.go:L99-120`:
```go
func OMHasData(raw *omResponse) bool {
	if raw == nil {
		return false
	}
	if raw.Current.Temp != nil {
		return true
	}
	for _, v := range raw.Hourly.Temp {
		if v != nil {
			return true
		}
	}
	for _, v := range raw.Daily.TMax {
		if v != nil {
			return true
		}
	}
	return false
}
```
   - *Bug & Fix:* The BOM ACCESS-G endpoint occasionally responded with HTTP 200 containing null arrays. `OMHasData` detects empty payloads and triggers automated fallback to general Open-Meteo models.

---

## 7. TOP 5 REFLECTION-WORTHY MOMENTS

### 1. Concurrent Worker Pool Implementation
- **File:** `backend/internal/weather/source.go:L531-576`
- **What Happened:** Sequential forecast queries for national map markers caused $>3\text{ s}$ latency. Built an 8-worker pool using channels and `sync.WaitGroup`.
- **What Was Learned:** Practical CSP concurrency in Go, safe channel termination order (`wg.Wait()` before `close`), and quantifying parallel speedups.

### 2. Multi-Tiered Graceful Degradation Strategy
- **File:** `backend/internal/weather/source.go:L255-293`
- **What Happened:** Upstream API outages or null responses from BOM ACCESS-G risked breaking client dashboards. Built a fallback chain from memory cache to live BOM, live Open-Meteo, and stale SQLite snapshots.
- **What Was Learned:** Distributed systems resilience and using pointer checks (`*float64`) to handle nullability without runtime nil-pointer dereferences.

### 3. Resolving SQLite Database Lock Contention
- **File:** `database/database.go:L26-34` & `backend/internal/handlers/locations.go:L142-166`
- **What Happened:** Concurrent HTTP goroutines triggered SQLite database locking errors under multi-user traffic. Configured WAL mode with `SetMaxOpenConns(1)` and drained rows into memory before remote network I/O.
- **What Was Learned:** Database locking boundaries and the rule never to hold database cursors open across external network calls.

### 4. Struct Embedding & Modular Decoupling
- **File:** `database/models.go:L46-49` & `backend/internal/weather/aliases.go:L10-20`
- **What Happened:** Splitting the codebase into packages caused circular import cycles between database models and domain logic. Solved via struct embedding (`UserWithPrefs`) and Go type aliases.
- **What Was Learned:** Go's compositional type system, avoiding circular imports, and achieving code reuse without traditional class inheritance.

### 5. Deterministic Evaluation Engine Using Functional Closures
- **File:** `backend/internal/weather/intelligence.go:L132-215`
- **What Happened:** Evaluated 10 outdoor activities against meteorological parameters using an inner closure `add := func(...)` to mutate and accumulate score factors within lexical scope.
- **What Was Learned:** How functional closures simplify accumulator patterns, eliminate parameter bloat, and produce explainable scoring outputs.
