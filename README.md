# QJS - JavaScript in Go with QuickJS and Wazero

[![Go Version](https://img.shields.io/badge/go-1.23.0+-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://godoc.org/github.com/fastschema/qjs?status.svg)](https://pkg.go.dev/github.com/fastschema/qjs)

QJS is a CGO-Free, modern, secure JavaScript runtime for Go applications, built on the powerful QuickJS engine and Wazero WebAssembly runtime. It allows you to run JavaScript code safely and efficiently, with full support for ES6+ features, async/await, and Go-JS interoperability.

## Features

- **JavaScript ES6+ Support**: Full ECMAScript 2020 compatibility via QuickJS
- **WebAssembly Execution**: Secure, sandboxed runtime using Wazero
- **Go-JS Interoperability**: Seamless data conversion between Go and JavaScript
- **Function Binding**: Expose Go functions to JavaScript and vice versa
- **Module System**: Support for ES6 modules and CommonJS
- **Async/Await**: Full support for asynchronous JavaScript execution
- **Runtime Pooling**: Reuse JavaScript runtimes for better performance
- **Memory Safety**: Memory-safe execution environment with configurable limits
- **No CGO Dependencies**: Pure Go implementation with WebAssembly

## Example Usage

```go
package main

import (
    "fmt"
    "github.com/fastschema/qjs"
)

func main() {
    // Create secure JavaScript runtime
    rt, _ := qjs.New()
    defer rt.Close()
    
    // Execute modern JavaScript with confidence
    result, _ := rt.Eval("demo.js", qjs.Code(`
        const users = [
            { name: "Alice", age: 30 },
            { name: "Bob", age: 25 }
        ];
        
        const adults = users
            .filter(u => u.age >= 18)
            .map(u => u.name.toUpperCase());
            
        ({ count: adults.length, adults })
    `))
    defer result.Free()
    
    fmt.Printf("Found %d adults: %v\n", 
        result.GetPropertyStr("count").Int32(),
        result.GetPropertyStr("adults"))
}
```

**Output:**
```
Found 2 adults: ["ALICE", "BOB"]
```

## Installation

```bash
go get github.com/fastschema/qjs
```


```go
import "github.com/fastschema/qjs"
```

**Compatible with Go 1.23.0+**

## Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Your Go   │ ---> │   Wazero    │ ---> │   QuickJS   │
│ Application │      │ WebAssembly │      │ JavaScript  │
│             │      │  Runtime    │      │   Engine    │
└─────────────┘      └─────────────┘      └─────────────┘
       ^                    ^                     ^
       │                    │                     │
   Structured           Sandboxed              ES2020+
   Go Types               Memory              Modern JS
```

## Core Concepts: Go - JavaScript Made Simple

### 1. Basic JavaScript Execution

```go
rt, err := qjs.New()
if err != nil {
    log.Fatal(err)
}
defer rt.Close()

// Execute any modern JavaScript
result, err := rt.Eval("math.js", qjs.Code(`
    Math.sqrt(16) + Math.pow(2, 3)
`))
defer result.Free()

fmt.Println("Result:", result.Float64()) // Output: 12
```

### 2. Exposing Go Functions to JavaScript

```go
ctx := rt.Context()

// Register a Go function
ctx.SetFunc("greet", func(this *qjs.This) (*qjs.Value, error) {
    args := this.Args()
    if len(args) == 0 {
        return this.Context().NewString("Hello, World!"), nil
    }
    name := args[0].String()
    return this.Context().NewString("Hello, " + name + "!"), nil
})

// Call it from JavaScript
result, _ := rt.Eval("greeting.js", qjs.Code(`greet("Alice")`))
fmt.Println(result.String()) // Output: Hello, Alice!
```

### 3. Working with Complex Data

```go
// Go struct to JavaScript object
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

user := User{Name: "Bob", Age: 30}
userJS, _ := qjs.ToJSValue(ctx, user)

ctx.Global().SetPropertyStr("user", userJS)

result, _ := rt.Eval("user-demo.js", qjs.Code(`
    user.name + " is " + user.age + " years old"
`))
fmt.Println(result.String()) // Output: Bob is 30 years old
```

## Advanced Features

### Async/Await Support

```go
// Register async Go function
ctx.SetAsyncFunc("fetchData", func(this *qjs.This) (*qjs.Value, error) {
    // Simulate async operation
    go func() {
        time.Sleep(100 * time.Millisecond)
        this.Promise().Resolve(this.Context().NewString("data loaded"))
    }()
    return this.Promise(), nil
})

// Use in async JavaScript
result, _ := rt.Eval("async.js", qjs.Code(`
    async function main() {
        const data = await fetchData();
        return "Got: " + data;
    }
    main()
`), qjs.FlagAsync())
```

### ES6 Modules

```go
// Load module without executing
rt.Load("math-utils.js", qjs.Code(`
    export function add(a, b) { return a + b; }
    export function multiply(a, b) { return a * b; }
`))

// Use module in another script
result, _ := rt.Eval("calculator.js", qjs.Code(`
    import { add, multiply } from 'math-utils.js';
    add(5, multiply(3, 4))
`), qjs.TypeModule())

fmt.Println(result.Int32()) // Output: 17
```

## API Reference

### Core Types

| Type | Description |
|------|-------------|
| `Runtime` | Main JavaScript runtime instance |
| `Context` | JavaScript execution context |
| `Value` | JavaScript value wrapper |
| `Pool` | Runtime pool for performance |

### Key Methods

```go
// Runtime Management
rt, err := qjs.New(options...)           // Create runtime
rt.Close()                               // Cleanup runtime
rt.Eval(filename, code, flags...)        // Execute JavaScript
rt.Load(filename, code)                  // Load module
rt.Compile(filename, code)               // Compile to bytecode
...

// Context Operations  
ctx := rt.Context()                      // Get context
ctx.Global()                             // Access global object
ctx.SetFunc(name, fn)                    // Bind Go function
ctx.SetAsyncFunc(name, fn)               // Bind async function
ctx.NewString(s)                         // Create JS string
ctx.NewObject()                          // Create JS object
...

// Value Operations
value.String()                           // Convert to Go string
value.Int32()                            // Convert to Go int32
value.Bool()                             // Convert to Go bool
value.GetPropertyStr(name)               // Get object property
value.SetPropertyStr(name, val)          // Set object property
value.Free()                             // Release memory
...
```

### Configuration Options

```go
type Option struct {
    CWD                string  // Working directory
    MaxStackSize       int     // Stack size limit
    MemoryLimit        int     // Memory usage limit  
    MaxExecutionTime   int     // Execution timeout
    GCThreshold        int     // GC trigger threshold
}
```

## Performance & Security

**Optimization Tips:**
1. Use runtime pools for concurrent applications
2. Compile frequently-used scripts to bytecode
3. Minimize large object conversions between Go and JS
4. Set appropriate memory limits

**Security**

- **Complete filesystem isolation** (unless explicitly configured)  
- **No network access** from JavaScript (unless explicitly allowed)
- **Memory safe** - no buffer overflows  
- **No CGO attack surface**  
- **Deterministic resource cleanup**  

### Memory Management

**Critical Rules:**
- Always call `result.Free()` on JavaScript values
- Always call `rt.Close()` when done with runtime
- Don't free functions registered to global object
- Don't free object properties directly – free the entire object

```go
// Correct pattern
result, err := rt.Eval("script.js", code)
if err != nil {
    return err
}
defer result.Free() // Always free values

// Wrong - will cause memory leaks
result, _ := rt.Eval("script.js", code)
// Missing result.Free()
```

**Choose QJS when you need:**
- Modern JavaScript features with security
- Zero external dependencies
- Plugin systems or user-generated code
- Compliance with strict security requirements

**Choose alternatives when:**
- **v8go**: Maximum performance and you can manage CGO dependencies
- **goja**: Simple scripts and don't need modern JS features
- **otto**: Legacy JavaScript and minimal dependencies

## Building from Source

### Prerequisites

- Go 1.23.0+
- WASI SDK (for WebAssembly compilation)
- CMake 3.16+
- Make

### Quick Build

```bash
# Clone with submodules
git clone --recursive https://github.com/fastschema/qjs.git
cd qjs

# Install WASI SDK (Linux/macOS)
curl -L https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-20/wasi-sdk-20.0-linux.tar.gz | tar xz
sudo mv wasi-sdk-20.0 /opt/wasi-sdk

# Build WebAssembly module
make build

# Run tests
go test ./...
```

### Development Workflow

```bash
# Build and test
make build && go test ./...

# Run benchmarks
go test -bench=. ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

```

## Contributing

We'd love your help making QJS better! Here's how:

1. **Found a bug?** [Open an issue](https://github.com/fastschema/qjs/issues)
2. **Want a feature?** Start a discussion
3. **Ready to code?** Fork, branch, test, and submit a PR

**Development Setup:**
```bash
git clone --recursive https://github.com/fastschema/qjs.git
cd qjs
make build
go test ./...
```

**Code Standards:**
- Follow standard Go conventions (`gofmt`, `golangci-lint`)
- Add tests for new features  
- Update documentation for API changes
- Keep commit messages clear and descriptive

## Support & Community

- **Documentation**: [GoDoc](https://godoc.org/github.com/fastschema/qjs)
- **Issues**: [GitHub Issues](https://github.com/fastschema/qjs/issues)
- **Discussions**: [GitHub Discussions](https://github.com/fastschema/qjs/discussions)

**Getting Help:**
1. Check existing issues and documentation
2. Create a minimal reproduction case
3. Include Go version, OS, and QJS version
4. Be specific about expected vs actual behavior



## License

MIT License - see [LICENSE](LICENSE) file.

## Acknowledgments

Built on the shoulders of giants:

- **[QuickJS](https://bellard.org/quickjs/)** by Fabrice Bellard - The elegant JavaScript engine
- **[Wazero](https://wazero.io/)** - Pure Go WebAssembly runtime  
- **[QuickJS-NG](https://github.com/quickjs-ng/quickjs)** - Maintained QuickJS fork

---

**Ready to run JavaScript safely in your Go apps?**

```bash
go get github.com/fastschema/qjs
```

**Questions? Ideas? Contributions?** We're here to help → [Start a discussion](https://github.com/fastschema/qjs/discussions)
