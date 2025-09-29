# QJS - JavaScript in Go with QuickJS and Wazero

<p align="center">
  <a href="https://pkg.go.dev/github.com/fastschema/qjs#section-readme" target="_blank" rel="noopener">
    <img src="https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white" alt="Go.Dev reference" />
  </a>
  <a href="https://goreportcard.com/report/github.com/fastschema/qjs" target="_blank" rel="noopener">
    <img src="https://goreportcard.com/badge/github.com/fastschema/qjs" alt="go report card" />
  </a>
<a href="https://app.fossa.com/projects/git%2Bgithub.com%2Ffastschema%2Fqjs?ref=badge_shield" alt="FOSSA Status"><img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2Ffastschema%2Fqjs.svg?type=shield"/></a>
  <a href="https://codecov.io/gh/fastschema/qjs/branch/master" >
    <img src="https://codecov.io/gh/fastschema/qjs/branch/master/graph/badge.svg?token=yluqOtL5z0"/>
  </a>
  <a href="https://github.com/fastschema/qjs/actions" target="_blank" rel="noopener">
    <img src="https://github.com/fastschema/qjs/actions/workflows/ci.yml/badge.svg" alt="test status" />
  </a>
  <a href="https://opensource.org/licenses/MIT" target="_blank" rel="noopener">
    <img src="https://img.shields.io/badge/license-MIT-brightgreen.svg" alt="MIT license" />
  </a>
</p>

QJS is a CGO-Free, modern, secure JavaScript runtime for Go applications, built on the powerful QuickJS engine and Wazero WebAssembly runtime. It allows you to run JavaScript code safely and efficiently, with full support for ES6+ features, async/await, and Go-JS interoperability.


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Ffastschema%2Fqjs.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Ffastschema%2Fqjs?ref=badge_large)

## Features

- **JavaScript ES6+ Support**: Full ECMAScript 2020 compatibility via QuickJS
- **WebAssembly Execution**: Secure, sandboxed runtime using Wazero
- **Go-JS Interoperability**: Seamless data conversion between Go and JavaScript
- **ProxyValue Support**: Zero-copy sharing of Go values with JavaScript via lightweight proxies
- **Function Binding**: Expose Go functions to JavaScript and vice versa
- **Module System**: Support for ES6 modules and CommonJS
- **Async/Await**: Full support for asynchronous JavaScript execution
- **Runtime Pooling**: Reuse JavaScript runtimes for better performance
- **Memory Safety**: Memory-safe execution environment with configurable limits
- **No CGO Dependencies**: Pure Go implementation with WebAssembly

## Example Usage


### Basic Execution

```go
rt, err := qjs.New()
if err != nil {
    panic(err)
}

defer rt.Close()
ctx := rt.Context()

result, err := ctx.Eval("test.js", qjs.Code(`
    const person = {
        name: "Alice",
        age: 30,
        city: "New York"
    };

    const info = Object.keys(person).map(key =>
        key + ": " + person[key]
    ).join(", ");

    ({ person: person, info: info });
`))
if err != nil {
    log.Fatal("Eval error:", err)
}
defer result.Free()
// Output: name: Alice, age: 30, city: New York
log.Println(result.GetPropertyStr("info").String())
// Output: Alice
log.Println(result.GetPropertyStr("person").GetPropertyStr("name").String())
// Output: 30
log.Println(result.GetPropertyStr("person").GetPropertyStr("age").Int32())
```

### Go function binding

```go
ctx.SetFunc("goFunction", func(this *qjs.This) (*qjs.Value, error) {
    return this.Context().NewString("Hello from Go!"), nil
})

result, err := ctx.Eval("test.js", qjs.Code(`
    const message = goFunction();
    message;
`))
if err != nil {
    panic(err)
}
defer result.Free()

// Output: Hello from Go!
log.Println(result.String())
```

### Async operations

**Awaiting a promise**

```go
ctx.SetAsyncFunc("asyncFunction", func(this *qjs.This) {
    go func() {
        time.Sleep(100 * time.Millisecond)
        result := this.Context().NewString("Async result from Go!")
        this.Promise().Resolve(result)
    }()
})

result, err := ctx.Eval("test.js", qjs.Code(`
    async function main() {
        const result = await asyncFunction();
        return result;
    }
    main()
`))

if err != nil {
    log.Fatal("Eval error:", err)
}
defer result.Free()

// Wait for the promise to resolve
result.Await()
// Output: Async result from Go!
log.Println(result.String())
```

**Top level await**

```go
result, err := ctx.Eval("test.js", qjs.Code(`
    async function main() {
        const result = await asyncFunction();
        return result;
    }
    await main()
`), qjs.FlagAsync())

if err != nil {
    log.Fatal("Eval error:", err)
}

defer result.Free()
log.Println(result.String())
```

### Call JS function from Go

```go
// Call JS function from Go
result, err := ctx.Eval("test.js", qjs.Code(`
    function add(a, b) {
        return a + b;
    }

    function errorFunc() {
        throw new Error("test error");
    }

    ({
        addFunc: add,
        errorFunc: errorFunc
    });
`))

if err != nil {
    panic(err)
}
defer result.Free()

jsAddFunc := result.GetPropertyStr("addFunc")
defer jsAddFunc.Free()

goAddFunc, err := qjs.JsFuncToGo[func(int, int) (int, error)](jsAddFunc)
if err != nil {
    panic(err)
}

total, err := goAddFunc(1, 2)
if err != nil {
    panic(err)
}

// Output: 3
log.Println(total)

jsErrorFunc := result.GetPropertyStr("errorFunc")
defer jsErrorFunc.Free()

goErrorFunc, err := qjs.JsFuncToGo[func() (any, error)](jsErrorFunc)
if err != nil {
    panic(err)
}

_, err = goErrorFunc()
if err != nil {
    // Output:
    // JS function execution failed: Error: test error
    // 	at errorFunc (test.js:7:14)
    log.Println(err.Error())
}
```

### ES Modules

```go
// Load a utility module
if _, err = ctx.Load("math-utils.js", qjs.Code(`
    export function add(a, b) {
        return a + b;
    }

    export function multiply(a, b) {
        return a * b;
    }

    export function power(base, exponent) {
        return Math.pow(base, exponent);
    }

    export const PI = 3.14159;
    export const E = 2.71828;
    export default {
        add,
        multiply,
        power,
        PI,
        E
    };
`)); err != nil {
    panic(err)
}

// Use the module
result, err := ctx.Eval("use-math.js", qjs.Code(`
    import mathUtils, { add, multiply, power, PI } from 'math-utils.js';

    const calculations = {
        addition: add(10, 20),
        multiplication: multiply(6, 7),
        power: power(2, 8),
        circleArea: PI * power(5, 2),
        defaultAdd: mathUtils.add(10, 20)
    };

    export default calculations;
`), qjs.TypeModule())

if err != nil {
    log.Fatal("Module eval error:", err)
}

// Output:
// Addition: 30
// Multiplication: 42
// Power: 256
// Circle Area: 78.54
// Default Add: 30
fmt.Printf("Addition: %d\n", result.GetPropertyStr("addition").Int32())
fmt.Printf("Multiplication: %.0f\n", result.GetPropertyStr("multiplication").Float64())
fmt.Printf("Power: %.0f\n", result.GetPropertyStr("power").Float64())
fmt.Printf("Circle Area: %.2f\n", result.GetPropertyStr("circleArea").Float64())
fmt.Printf("Default Add: %.d\n", result.GetPropertyStr("defaultAdd").Int32())
result.Free()
```

### Bytecode Compilation

```go
script := `
    function fibonacci(n) {
        if (n <= 1) return n;
        return fibonacci(n - 1) + fibonacci(n - 2);
    }

    function factorial(n) {
        return n <= 1 ? 1 : n * factorial(n - 1);
    }

    const result = {
        fib10: fibonacci(10),
        fact5: factorial(5),
        timestamp: Date.now()
    };

    result;
`

// Compile the script to bytecode
bytecode, err := ctx.Compile("math-functions.js", qjs.Code(script))
if err != nil {
    log.Fatal("Compilation error:", err)
}

fmt.Printf("Bytecode size: %d bytes\n", len(bytecode))

// Execute the compiled bytecode
result, err := ctx.Eval("compiled-math.js", qjs.Bytecode(bytecode))
if err != nil {
    log.Fatal("Bytecode execution error:", err)
}

fmt.Printf("Fibonacci(10): %d\n", result.GetPropertyStr("fib10").Int32())
fmt.Printf("Factorial(5): %d\n", result.GetPropertyStr("fact5").Int32())
result.Free()
```

### ProxyValue Support

ProxyValue is a feature that allows you to pass Go values directly to JavaScript without full serialization, enabling efficient sharing of complex objects, functions, and resources.

ProxyValue creates a lightweight JavaScript wrapper around Go values, storing only a reference ID rather than copying the entire value. This is particularly useful for **pass-through scenarios** where JavaScript receives a Go value and passes it back to Go without needing to access its contents.

Key benefits:
- **Zero-copy data sharing** - no serialization/deserialization overhead
- **Pass-through efficiency** - JavaScript can hold and return Go values without conversion
- **Type preservation** - original Go types are maintained across boundaries
- **Resource efficiency** - perfect for objects like `context.Context`, database connections, or large structs

**Most common use case**: JavaScript callbacks that receive Go values (like `context.Context`) and pass them back to Go functions without ever accessing the value contents in JavaScript.

#### Basic ProxyValue Usage

```go
package main

import (
	"context"
	"fmt"

	"github.com/fastschema/qjs"
)

func main() {
	rt, err := qjs.New()
	if err != nil {
		panic(err)
	}
	defer rt.Close()
	ctx := rt.Context()

	// Create a Go function that accepts context and a number
	goFuncWithContext := func(ctx context.Context, num int) int {
		// Access context values in Go
		fmt.Printf("Context received: %v\n", ctx)
		return num * 2
	}

	// Convert Go function to JavaScript function
	jsFuncWithContext, err := qjs.ToJSValue(ctx, goFuncWithContext)
	if err != nil {
		panic(err)
	}
	defer jsFuncWithContext.Free()
	ctx.Global().SetPropertyStr("funcWithContext", jsFuncWithContext)

	// Create a helper function that returns a ProxyValue
	ctx.SetFunc("$context", func(this *qjs.This) (*qjs.Value, error) {
		// Create context as ProxyValue - JavaScript will never access its contents
		val := ctx.NewProxyValue(context.Background())
		return val, nil
	})

	// JavaScript gets context as ProxyValue and passes it to Go function
	result, err := ctx.Eval("test.js", qjs.Code(`
		funcWithContext($context(), 10);
	`))
	if err != nil {
		panic(err)
	}
	defer result.Free()

	// Output: 20
	fmt.Println(result.Int32())
}
```

### GO-JS Conversion

```go
package main

import (
	"fmt"

	"github.com/fastschema/qjs"
)

type Post struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Author User   `json:"author"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Method on User struct
func (u User) GetDisplayName() string {
	return fmt.Sprintf("%s (%d)", u.Name, u.Age)
}

func (u User) IsAdult() bool {
	return u.Age >= 18
}

func main() {
	rt, err := qjs.New()
	if err != nil {
		panic(err)
	}
	defer rt.Close()
	ctx := rt.Context()

	ctx.Global().SetPropertyStr("goInt", ctx.NewInt32(55))
	ctx.Global().SetPropertyStr("goString", ctx.NewString("Hello, World!"))
	jsUser, err := qjs.ToJSValue(ctx, User{ID: 1, Name: "Alice", Age: 25})
	if err != nil {
		panic(err)
	}
	ctx.Global().SetPropertyStr("goUser", jsUser)

	result, err := ctx.Eval("test.js", qjs.Code(`
		const post = {
			id: goInt,
			name: goString,
			author: goUser
		};
		post;
	`))
	if err != nil {
		panic(err)
	}
	defer result.Free()

	goPost, err := qjs.JsValueToGo[Post](result)
	if err != nil {
		panic(err)
	}

	// Output:
	// Post ID: 55
	// Post Name: Hello, World!
	// Author ID: 1
	// Author Name: Alice
	// Author Age: 25
	fmt.Printf("Post ID: %d\n", goPost.ID)
	fmt.Printf("Post Name: %s\n", goPost.Name)
	fmt.Printf("Author ID: %d\n", goPost.Author.ID)
	fmt.Printf("Author Name: %s\n", goPost.Author.Name)
	fmt.Printf("Author Age: %d\n", goPost.Author.Age)
}
```

### Pool

```go
package main

import (
	"log"
	"sync"

	"github.com/fastschema/qjs"
)

func main() {
	setupFunc := func(rt *qjs.Runtime) error {
		ctx := rt.Context()
		ctx.Eval("setup.js", qjs.Code(`
			function getMessage(workerId, taskId) {
				return "Hello from pooled runtime: " + workerId + "-" + taskId;
			}
		`))
		return nil
	}
	// Create a pool with 3 runtimes
	pool := qjs.NewPool(3, &qjs.Option{}, setupFunc)
	numWorkers := 5
	numTasks := 3
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numTasks; j++ {
				rt, err := pool.Get()
				if err != nil {
					panic(err)
				}
				defer pool.Put(rt)
				ctx := rt.Context()
				workerIdValue := ctx.NewInt32(int32(workerID))
				taskIdValue := ctx.NewInt32(int32(j))
				ctx.Global().SetPropertyStr("workerID", workerIdValue)
				ctx.Global().SetPropertyStr("taskID", taskIdValue)

				// Use the runtime
				result, err := ctx.Eval("pool-test.js", qjs.Code(`
					({
						message: getMessage(workerID, taskID),
						timestamp: Date.now(),
					});
				`))
				if err != nil {
					panic(err)
				}
				defer result.Free()
				log.Println(result.GetPropertyStr("message").String())
			}
		}(i)
	}
	wg.Wait()
}
```

## Installation

```bash
go get github.com/fastschema/qjs
```


```go
import "github.com/fastschema/qjs"
```

**Compatible with Go 1.22.0+**

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

## API Reference

### Core Types

| Type | Description |
|------|-------------|
| `Runtime` | Main JavaScript runtime instance |
| `Context` | JavaScript execution context |
| `Value` | JavaScript value wrapper |
| `Pool` | Runtime pool for performance |
| `ProxyRegistry` | Thread-safe registry for ProxyValue objects |

### Key Methods

```go
// Runtime Management
rt, err := qjs.New(options...)           // Create runtime
rt.Close()                               // Cleanup runtime
ctx.Eval(filename, code, flags...)        // Execute JavaScript
rt.Load(filename, code)                  // Load module
rt.Compile(filename, code)               // Compile to bytecode
...

// Context Operations  
ctx := ctx                      // Get context
ctx.Global()                             // Access global object
ctx.SetFunc(name, fn)                    // Bind Go function
ctx.SetAsyncFunc(name, fn)               // Bind async function
ctx.NewString(s)                         // Create JS string
ctx.NewObject()                          // Create JS object
ctx.NewProxyValue(v)                     // Create ProxyValue from Go value
...

// Value Operations
value.String()                           // Convert to Go string
value.Int32()                            // Convert to Go int32
value.Bool()                             // Convert to Go bool
value.GetPropertyStr(name)               // Get object property
value.SetPropertyStr(name, val)          // Set object property
value.IsQJSProxyValue()                  // Check if value is a ProxyValue
value.Free()                             // Release memory
...

// ProxyValue Operations
qjs.JsValueToGo[T](value)               // Extract Go value from ProxyValue
qjs.ToJSValue(ctx, goValue)             // Convert Go value to JS (auto-detects ProxyValue need)
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
3. Use ProxyValue for large objects or shared state to avoid serialization overhead
4. Minimize small object conversions between Go and JS - prefer ProxyValue for complex types
5. Set appropriate memory limits

**Security**

- **Complete filesystem isolation** (unless explicitly configured)  
- **No network access** from JavaScript (unless explicitly allowed)
- **Memory safe** - no buffer overflows  
- **No CGO attack surface**  
- **Deterministic resource cleanup**
- **ProxyValue safety** - Go values are protected by type checking and automatic cleanup

### Memory Management

**Critical Rules:**
- Always call `result.Free()` on JavaScript values
- Always call `rt.Close()` when done with runtime
- Don't free functions registered to global object
- Don't free object properties directly – free the entire object

```go
// Correct pattern
result, err := ctx.Eval("script.js", code)
if err != nil {
    return err
}
defer result.Free() // Always free values

// Wrong - will cause memory leaks
result, _ := ctx.Eval("script.js", code)
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