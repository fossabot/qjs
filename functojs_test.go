package qjs_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Calculator struct {
	Name string
}

func (c Calculator) Add(a, b int) int {
	return a + b
}

func (c Calculator) Multiply(a, b float64) (float64, error) {
	return a * b, nil
}

func (c Calculator) Sum(nums ...int) int {
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}

func (c Calculator) GetName() string {
	return c.Name
}

func (c *Calculator) SetName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	c.Name = name
	return nil
}

func createAndRegisterJSFunc(t *testing.T, ctx *qjs.Context, name string, fn any) *qjs.Value {
	jsFunc := must(qjs.FuncToJS(ctx, fn))
	assert.True(t, jsFunc.IsFunction())
	ctx.Global().SetPropertyStr(name, jsFunc)
	return jsFunc
}

func TestBasicConversion(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("FunctionTypes", func(t *testing.T) {
		// Simple function
		jsFunc := createAndRegisterJSFunc(t, ctx, "add", func(a, b int) int { return a + b })
		defer jsFunc.Free()
		result := must(ctx.Eval("test.js", qjs.Code("add(5, 3)")))
		defer result.Free()
		assert.Equal(t, int32(8), result.Int32())

		// Function with error
		jsFunc2 := createAndRegisterJSFunc(t, ctx, "divide", func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, errors.New("division by zero")
			}
			return a / b, nil
		})
		defer jsFunc2.Free()
		result2 := must(ctx.Eval("test.js", qjs.Code("divide(10, 2)")))
		defer result2.Free()
		assert.InEpsilon(t, float64(5), result2.Float64(), 0.0001)

		_, err := ctx.Eval("test.js", qjs.Code("divide(10, 0)"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "division by zero")

		// Error-only return
		jsFunc3 := createAndRegisterJSFunc(t, ctx, "errorOnly", func(shouldError bool) error {
			if shouldError {
				return errors.New("test error")
			}
			return nil
		})
		defer jsFunc3.Free()
		result3 := must(ctx.Eval("test.js", qjs.Code("errorOnly(false)")))
		defer result3.Free()
		assert.True(t, result3.IsUndefined())

		// No return value
		jsFunc4 := createAndRegisterJSFunc(t, ctx, "print", func(msg string) { fmt.Printf("Message: %s\n", msg) })
		defer jsFunc4.Free()
		result4 := must(ctx.Eval("test.js", qjs.Code(`print("hello")`)))
		defer result4.Free()
		assert.True(t, result4.IsUndefined())
	})

	t.Run("NilHandling", func(t *testing.T) {
		// Nil value
		result := must(qjs.FuncToJS(ctx, nil))
		defer result.Free()
		assert.True(t, result.IsNull())

		// Nil function
		var nilFunc func()
		jsFunc := must(qjs.FuncToJS(ctx, nilFunc))
		defer jsFunc.Free()
		assert.True(t, jsFunc.IsNull())

		// Nil function pointer
		var nilFuncPtr *func()
		jsNilPtr := must(qjs.FuncToJS(ctx, nilFuncPtr))
		defer jsNilPtr.Free()
		assert.True(t, jsNilPtr.IsNull())
	})

	t.Run("FunctionPointer", func(t *testing.T) {
		addFunc := func(a, b int) int { return a + b }
		funcPtr := &addFunc
		jsFunc := createAndRegisterJSFunc(t, ctx, "funcPtr", funcPtr)
		defer jsFunc.Free()
		result := must(ctx.Eval("test.js", qjs.Code("funcPtr(7, 8)")))
		defer result.Free()
		assert.Equal(t, int32(15), result.Int32())
	})
}

func TestParameters(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("InvalidParameterTypes", func(t *testing.T) {
		goFunc, err := qjs.FuncToJS(ctx, func(x int, y ...int) int {
			return len(y) + 1
		})
		assert.NoError(t, err)
		defer goFunc.Free()
		ctx.Global().SetPropertyStr("testFunc", goFunc)
		_, err = ctx.Eval("test.js", qjs.Code(`testFunc("invalid", 2)`))
		require.Error(t, err)
	})

	t.Run("BasicParameterTypes", func(t *testing.T) {
		_ = createAndRegisterJSFunc(t, ctx, "basicTypesFunc", func(s string, i int, f float64, b bool) string {
			return fmt.Sprintf("string:%s,int:%d,float:%.1f,bool:%t", s, i, f, b)
		})

		result := must(ctx.Eval("test.js", qjs.Code(`basicTypesFunc("test", 42, 3.14, true)`)))
		defer result.Free()
		assert.Equal(t, "string:test,int:42,float:3.1,bool:true", result.String())
	})

	t.Run("ComplexParameterTypes", func(t *testing.T) {
		t.Run("MapParameters", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "processData", func(data map[string]any) (string, error) {
				if len(data) == 0 {
					return "", errors.New("empty data")
				}
				return fmt.Sprintf("processed %d items, value=%v", len(data), data["value"]), nil
			})

			result := must(ctx.Eval("test.js", qjs.Code(`processData({name: "test", value: 42})`)))
			defer result.Free()
			assert.Equal(t, "processed 2 items, value=42", result.String())

			_, err := ctx.Eval("test.js", qjs.Code(`processData({})`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "empty data")
		})

		t.Run("ArrayAndSliceParameters", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "arrayFunc", func(arr [3]int) string {
				return fmt.Sprintf("array: %v", arr)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`arrayFunc([1, 2, 3])`)))
			defer result.Free()
			assert.Contains(t, result.String(), "array:")
		})
	})

	t.Run("PointerParameters", func(t *testing.T) {
		t.Run("BasicPointerHandling", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "pointerFunc", func(ptr *int) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %d", *ptr)
			})

			result1 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(42)`)))
			defer result1.Free()
			assert.Equal(t, "value: 42", result1.String())

			result2 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(null)`)))
			defer result2.Free()
			assert.Equal(t, "nil", result2.String())

			result3 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(undefined)`)))
			defer result3.Free()
			assert.Equal(t, "nil", result3.String())
		})

		t.Run("VariousPointerTypes", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "ptrStringFunc", func(ptr *string) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %s", *ptr)
			})

			result4 := must(ctx.Eval("test.js", qjs.Code(`ptrStringFunc("hello")`)))
			defer result4.Free()
			assert.Equal(t, "value: hello", result4.String())

			_ = createAndRegisterJSFunc(t, ctx, "ptrParamFunc", func(ptr *float64) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %.2f", *ptr)
			})

			result5 := must(ctx.Eval("test.js", qjs.Code(`ptrParamFunc(3.14159)`)))
			defer result5.Free()
			assert.Equal(t, "value: 3.14", result5.String())
		})

		t.Run("PointerConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "ptrFunc", func(ptr *string) string {
				if ptr == nil {
					return "nil"
				}
				return *ptr
			})

			_, err := ctx.Eval("test.js", qjs.Code(`ptrFunc(new Date())`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert JS function argument")
		})
	})

	t.Run("InterfaceParameters", func(t *testing.T) {
		t.Run("BasicInterfaceHandling", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "interfaceFunc", func(value any) string {
				return fmt.Sprintf("%T: %v", value, value)
			})

			tests := []struct {
				input    string
				contains string
			}{
				{`interfaceFunc(42)`, "42"},
				{`interfaceFunc("hello")`, "hello"},
				{`interfaceFunc(true)`, "true"},
				{`interfaceFunc({a: 1, b: "test"})`, "map"},
			}

			for _, test := range tests {
				result := must(ctx.Eval("test.js", qjs.Code(test.input)))
				defer result.Free()
				assert.Contains(t, result.String(), test.contains)
			}
		})

		t.Run("InterfaceTypeDetection", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "anyFunc", func(value any) string {
				return fmt.Sprintf("%T: %v", value, value)
			})

			// Test whole number conversion to int
			result := must(ctx.Eval("test.js", qjs.Code(`anyFunc(42)`)))
			defer result.Free()
			assert.Contains(t, result.String(), "int64: 42")

			// Test fractional number conversion to float64
			result2 := must(ctx.Eval("test.js", qjs.Code(`anyFunc(3.14)`)))
			defer result2.Free()
			assert.Contains(t, result2.String(), "float64")
			assert.Contains(t, result2.String(), "3.14")
		})
	})

	t.Run("FunctionParameters", func(t *testing.T) {
		t.Run("CallbackFunctions", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "higherOrderFunc", func(callback func(int) (int, error), value int) (int, error) {
				return callback(value)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`higherOrderFunc(x => x * 2, 21)`)))
			defer result.Free()
			assert.Equal(t, int32(42), result.Int32())

			_, err := ctx.Eval("test.js", qjs.Code(`higherOrderFunc(x => { throw new Error("callback error"); }, 10)`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "callback error")
		})

		t.Run("FunctionConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "callbackTest", func(callback func() error) error {
				return callback()
			})

			_, err2 := ctx.Eval("test.js", qjs.Code(`callbackTest(42)`))
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert JS function argument")
		})
	})

	t.Run("VariadicParameters", func(t *testing.T) {
		t.Run("BasicVariadicFunction", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "sumFunc", func(nums ...int) int {
				total := 0
				for _, num := range nums {
					total += num
				}
				return total
			})

			result1 := must(ctx.Eval("test.js", qjs.Code("sumFunc(1, 2, 3, 4, 5)")))
			defer result1.Free()
			assert.Equal(t, int32(15), result1.Int32())

			result2 := must(ctx.Eval("test.js", qjs.Code("sumFunc()")))
			defer result2.Free()
			assert.Equal(t, int32(0), result2.Int32())

			result3 := must(ctx.Eval("test.js", qjs.Code("sumFunc(42)")))
			defer result3.Free()
			assert.Equal(t, int32(42), result3.Int32())
		})

		t.Run("MixedVariadicFunction", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "partialSumFunc", func(prefix string, nums ...int) string {
				total := 0
				for _, num := range nums {
					total += num
				}
				return fmt.Sprintf("%s: %d", prefix, total)
			})

			result1 := must(ctx.Eval("test.js", qjs.Code(`partialSumFunc("Total", 1, 2, 3)`)))
			defer result1.Free()
			assert.Equal(t, "Total: 6", result1.String())

			result2 := must(ctx.Eval("test.js", qjs.Code(`partialSumFunc("Total")`)))
			defer result2.Free()
			assert.Equal(t, "Total: 0", result2.String())
		})

		t.Run("VariadicWithError", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "variadicWithError", func(prefix string, numbers ...int) (string, error) {
				if len(numbers) == 0 {
					return "", errors.New("no numbers provided")
				}
				sum := 0
				for _, num := range numbers {
					sum += num
				}
				return fmt.Sprintf("%s: %d", prefix, sum), nil
			})

			result := must(ctx.Eval("test.js", qjs.Code(`variadicWithError("Sum", 1, 2, 3, 4)`)))
			defer result.Free()
			assert.Equal(t, "Sum: 10", result.String())

			_, err := ctx.Eval("test.js", qjs.Code(`variadicWithError("Sum")`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no numbers provided")
		})

		t.Run("VariadicWithAnyType", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "formatFunc", func(format string, args ...any) string {
				return fmt.Sprintf(format, args...)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`formatFunc("Hello %s, you are %d years old", "John", 25)`)))
			defer result.Free()
			assert.Equal(t, "Hello John, you are 25 years old", result.String())
		})

		t.Run("VariadicConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "variadicIntFunc", func(nums ...int8) string {
				total := int8(0)
				for _, num := range nums {
					total += num
				}
				return fmt.Sprintf("total: %d", total)
			})

			_, err := ctx.Eval("test.js", qjs.Code(`variadicIntFunc(1, 2, 1000)`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert JS function argument")
		})
	})

	t.Run("UnsupportedParameterTypes", func(t *testing.T) {
		t.Run("ChannelParameters", func(t *testing.T) {
			_, err := qjs.FuncToJS(ctx, func(ch chan int) int {
				return <-ch
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert Go func param 'channel")
		})

		t.Run("UnsafePointerParameters", func(t *testing.T) {
			_, err2 := qjs.FuncToJS(ctx, func(ptr unsafe.Pointer) bool {
				return ptr != nil
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go func param 'unsafe.Pointer'")
		})

		t.Run("NestedUnsupportedTypes", func(t *testing.T) {
			_, err3 := qjs.FuncToJS(ctx, func(arr [3]chan int) {})
			require.Error(t, err3)
			assert.Contains(t, err3.Error(), "cannot convert Go func param 'array")

			_, err4 := qjs.FuncToJS(ctx, func(chans []chan int) int {
				return len(chans)
			})
			require.Error(t, err4)
			assert.Contains(t, err4.Error(), "cannot convert Go func param 'slice")

			_, err5 := qjs.FuncToJS(ctx, func(m map[string]chan int) int {
				return len(m)
			})
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "cannot convert Go func param 'map value: chan int' to JS")

			_, err6 := qjs.FuncToJS(ctx, func(m map[chan int]string) {})
			require.Error(t, err6)
			assert.Contains(t, err6.Error(), "cannot convert Go func param 'map key: chan int' to JS")

			type StructWithChan struct {
				Name string
				Ch   chan int
			}

			_, err7 := qjs.FuncToJS(ctx, func(s StructWithChan) string {
				return s.Name
			})
			require.Error(t, err7)
			assert.Contains(t, err7.Error(), "cannot convert Go 'qjs_test.StructWithChan.Ch' to JS")
			assert.Contains(t, err7.Error(), "cannot convert Go func param 'channel")
		})
	})
}

func TestReturnValues(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("BasicReturnTypes", func(t *testing.T) {
		t.Run("SingleReturn", func(t *testing.T) {
			jsFunc := createAndRegisterJSFunc(t, ctx, "nilFunc", func() any {
				return nil
			})
			defer jsFunc.Free()
			result := must(ctx.Eval("test.js", qjs.Code(`nilFunc()`)))
			defer result.Free()
			assert.True(t, result.IsNull())
		})

		t.Run("FunctionReturn", func(t *testing.T) {
			jsFunc := createAndRegisterJSFunc(t, ctx, "returningFunc", func() func(int) int {
				return func(x int) int {
					return x * 3
				}
			})
			defer jsFunc.Free()
			result := must(ctx.Eval("test.js", qjs.Code(`const func = returningFunc(); func(4)`)))
			defer result.Free()
			assert.Equal(t, int32(12), result.Int32())
		})
	})

	t.Run("ComplexReturnTypes", func(t *testing.T) {
		t.Run("StructReturn", func(t *testing.T) {
			type ComplexStruct struct {
				Name    string            `json:"name"`
				Values  []int             `json:"values"`
				Mapping map[string]string `json:"mapping"`
			}

			jsFunc := createAndRegisterJSFunc(t, ctx, "complexFunc", func() ComplexStruct {
				return ComplexStruct{
					Name:   "test",
					Values: []int{1, 2, 3},
					Mapping: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				}
			})
			defer jsFunc.Free()
			result := must(ctx.Eval("test.js", qjs.Code(`complexFunc()`)))
			defer result.Free()
			assert.True(t, result.IsObject())

			name := result.GetPropertyStr("name")
			defer name.Free()
			assert.Equal(t, "test", name.String())

			values := result.GetPropertyStr("values")
			defer values.Free()
			assert.True(t, values.IsArray())
		})

		t.Run("ArrayReturnTypes", func(t *testing.T) {
			jsFunc := createAndRegisterJSFunc(t, ctx, "arrayReturnFunc", func() [2]string {
				return [2]string{"hello", "world"}
			})
			defer jsFunc.Free()

			result := must(ctx.Eval("test.js", qjs.Code(`arrayReturnFunc()`)))
			defer result.Free()
			assert.True(t, result.IsArray())

			elem0 := result.GetPropertyIndex(0)
			defer elem0.Free()
			assert.Equal(t, "hello", elem0.String())

			elem1 := result.GetPropertyIndex(1)
			defer elem1.Free()
			assert.Equal(t, "world", elem1.String())
		})
	})

	t.Run("UnsupportedReturnTypes", func(t *testing.T) {
		t.Run("ChannelReturnTypes", func(t *testing.T) {
			_, err := qjs.FuncToJS(ctx, func() chan int {
				return make(chan int)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert Go func return 'channel")

			_, err2 := qjs.FuncToJS(ctx, func() map[chan int]string {
				return nil
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go func return 'map key: chan int' to JS")

			_, err3 := qjs.FuncToJS(ctx, func() []chan int {
				return nil
			})
			require.Error(t, err3)
			assert.Contains(t, err3.Error(), "cannot convert Go func return 'slice: chan int' to JS")
		})

		t.Run("UnsafePointerReturnTypes", func(t *testing.T) {
			_, err4 := qjs.FuncToJS(ctx, func() *unsafe.Pointer {
				return nil
			})
			require.Error(t, err4)
			assert.Contains(t, err4.Error(), "cannot convert Go func return 'unsafe.Pointer")

			_, err5 := qjs.FuncToJS(ctx, func() **chan int {
				return nil
			})
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "cannot convert Go func return 'channel")
		})

		t.Run("ArrayWithUnsupportedElements", func(t *testing.T) {
			_, err3 := qjs.FuncToJS(ctx, func() [2]chan int {
				return [2]chan int{}
			})
			require.Error(t, err3)
			assert.Contains(t, err3.Error(), "cannot convert Go func return 'array: chan int' to JS")
		})

		t.Run("StructWithUnsupportedFields", func(t *testing.T) {
			type StructWithChan struct {
				Name string
				Ch   chan int
			}
			_, err2 := qjs.FuncToJS(ctx, func() StructWithChan {
				return StructWithChan{Name: "test", Ch: make(chan int)}
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go 'qjs_test.StructWithChan.Ch' to JS")
			assert.Contains(t, err2.Error(), "cannot convert Go func return 'channel")

			// Nested validation
			type StructWithNestedChan struct {
				Field struct {
					ChanField chan int
				}
			}
			_, err5 := qjs.FuncToJS(ctx, func() StructWithNestedChan {
				return StructWithNestedChan{}
			})
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "cannot convert Go 'qjs_test.StructWithNestedChan.Field' to JS")
			assert.Contains(t, err5.Error(), "cannot convert Go func return 'channel")
		})

		t.Run("ComplexNestedValidation", func(t *testing.T) {
			_, err := qjs.FuncToJS(ctx, func() map[string]unsafe.Pointer {
				return nil
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert Go func return 'map value: unsafe.Pointer' to JS")

			type StructWithUnsafeField struct {
				Field unsafe.Pointer
			}
			_, err2 := qjs.FuncToJS(ctx, func() StructWithUnsafeField {
				return StructWithUnsafeField{}
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go 'qjs_test.StructWithUnsafeField.Field' to JS")
			assert.Contains(t, err2.Error(), "cannot convert Go func return 'unsafe.Pointer'")
		})
	})

	t.Run("RecursiveTypeHandling", func(t *testing.T) {
		type RecursiveStruct struct {
			Name string           `json:"name"`
			Next *RecursiveStruct `json:"next"`
		}

		jsFunc := createAndRegisterJSFunc(t, ctx, "recursiveFunc", func(r RecursiveStruct) string {
			return r.Name
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`recursiveFunc({name: "test", next: null})`)))
		defer result.Free()
		assert.Equal(t, "test", result.String())
	})

	t.Run("ErrorHandlingInResults", func(t *testing.T) {
		t.Run("ErrorResultHandling", func(t *testing.T) {
			runtime := must(qjs.New())
			defer runtime.Close()
			testCtx := runtime.Context()

			jsFunc := createAndRegisterJSFunc(t, testCtx, "errorFunc", func() (string, error) {
				return "test", errors.New("test error")
			})
			defer jsFunc.Free()

			_, err := testCtx.Eval("test.js", qjs.Code(`errorFunc()`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test error")

			jsFunc2 := createAndRegisterJSFunc(t, testCtx, "nilErrorFunc", func() error {
				return nil
			})
			defer jsFunc2.Free()

			result := must(testCtx.Eval("test.js", qjs.Code(`nilErrorFunc()`)))
			defer result.Free()
			assert.True(t, result.IsUndefined())
		})

		t.Run("HandleResultsFallbackPath", func(t *testing.T) {
			runtime := must(qjs.New())
			defer runtime.Close()
			testCtx := runtime.Context()

			results := []reflect.Value{
				reflect.ValueOf("test result"),
				reflect.ValueOf("not an error"),
			}

			jsValue, err := qjs.GoFuncResultToJs(testCtx, results)
			assert.NoError(t, err)
			defer jsValue.Free()
			assert.Equal(t, "test result", jsValue.String())
		})
	})
}

func TestNumericTypes(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("AllNumericTypes", func(t *testing.T) {
		t.Run("IntegerTypes", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "intFunc", func(a int, b int32, c int64) string {
				return fmt.Sprintf("int:%d, int32:%d, int64:%d", a, b, c)
			})

			result1 := must(ctx.Eval("test.js", qjs.Code(`intFunc(42, 25, 100)`)))
			defer result1.Free()
			assert.Equal(t, "int:42, int32:25, int64:100", result1.String())
		})

		t.Run("FloatTypes", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "floatFunc", func(a float32, b float64) string {
				return fmt.Sprintf("float32:%.1f, float64:%.1f", a, b)
			})

			result2 := must(ctx.Eval("test.js", qjs.Code(`floatFunc(3.14, 2.718)`)))
			defer result2.Free()
			assert.Equal(t, "float32:3.1, float64:2.7", result2.String())
		})

		t.Run("UnsignedIntegerTypes", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "uintFunc", func(a uint, b uint32, c uint64) string {
				return fmt.Sprintf("uint:%d, uint32:%d, uint64:%d", a, b, c)
			})

			result3 := must(ctx.Eval("test.js", qjs.Code(`uintFunc(42, 25, 100)`)))
			defer result3.Free()
			assert.Equal(t, "uint:42, uint32:25, uint64:100", result3.String())
		})
	})

	t.Run("NumericOverflowValidation", func(t *testing.T) {
		t.Run("Int8Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "int8Func", func(value int8) string {
				return fmt.Sprintf("int8:%d", value)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`int8Func(100)`)))
			defer result.Free()
			assert.Equal(t, "int8:100", result.String())

			_, err := ctx.Eval("test.js", qjs.Code(`int8Func(1000)`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "overflow")

			_, err2 := ctx.Eval("test.js", qjs.Code(`int8Func(-129)`))
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "overflows int8")
		})

		t.Run("Uint8Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "uint8Func", func(val uint8) string {
				return fmt.Sprintf("uint8: %d", val)
			})

			result4 := must(ctx.Eval("test.js", qjs.Code(`uint8Func(200)`)))
			defer result4.Free()
			assert.Equal(t, "uint8: 200", result4.String())

			_, err7 := ctx.Eval("test.js", qjs.Code(`uint8Func(256)`))
			require.Error(t, err7)
			assert.Contains(t, err7.Error(), "overflows uint8")
		})

		t.Run("Int16Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "int16Func", func(val int16) string {
				return fmt.Sprintf("int16: %d", val)
			})

			result2 := must(ctx.Eval("test.js", qjs.Code(`int16Func(32000)`)))
			defer result2.Free()
			assert.Equal(t, "int16: 32000", result2.String())

			_, err3 := ctx.Eval("test.js", qjs.Code(`int16Func(32768)`))
			require.Error(t, err3)
			assert.Contains(t, err3.Error(), "overflows int16")

			_, err4 := ctx.Eval("test.js", qjs.Code(`int16Func(-32769)`))
			require.Error(t, err4)
			assert.Contains(t, err4.Error(), "overflows int16")
		})

		t.Run("Uint16Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "uint16Func", func(val uint16) string {
				return fmt.Sprintf("uint16: %d", val)
			})

			result5 := must(ctx.Eval("test.js", qjs.Code(`uint16Func(60000)`)))
			defer result5.Free()
			assert.Equal(t, "uint16: 60000", result5.String())

			_, err8 := ctx.Eval("test.js", qjs.Code(`uint16Func(65536)`))
			require.Error(t, err8)
			assert.Contains(t, err8.Error(), "overflows uint16")
		})

		t.Run("Int32Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "int32Func", func(val int32) string {
				return fmt.Sprintf("int32: %d", val)
			})

			_, err5 := ctx.Eval("test.js", qjs.Code(`int32Func(2147483648)`))
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "overflow")

			_, err6 := ctx.Eval("test.js", qjs.Code(`int32Func(-2147483649)`))
			require.Error(t, err6)
			assert.Contains(t, err6.Error(), "overflow")
		})

		t.Run("Uint32Overflow", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "uint32Func", func(val uint32) string {
				return fmt.Sprintf("uint32: %d", val)
			})

			_, err9 := ctx.Eval("test.js", qjs.Code(`uint32Func(-1)`))
			require.Error(t, err9)
			assert.Contains(t, err9.Error(), "overflow")

			_, err10 := ctx.Eval("test.js", qjs.Code(`uint32Func(4294967296)`))
			require.Error(t, err10)
			assert.Contains(t, err10.Error(), "overflow")
		})
	})

	t.Run("ComplexNumbers", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "complexFunc", func(c64 complex64, c128 complex128) string {
			return fmt.Sprintf("complex64:%.1f, complex128:%.1f", real(c64), real(c128))
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`complexFunc(42.5, 100.7)`)))
		defer result.Free()
		assert.Equal(t, "complex64:42.5, complex128:100.7", result.String())
	})
}

func TestMethodBinding(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("StructMethodBinding", func(t *testing.T) {
		t.Run("ValueReceiverMethods", func(t *testing.T) {
			calc := Calculator{Name: "MyCalculator"}

			jsAddMethod := createAndRegisterJSFunc(t, ctx, "calcAdd", calc.Add)
			defer jsAddMethod.Free()
			result := must(ctx.Eval("test.js", qjs.Code("calcAdd(10, 20)")))
			defer result.Free()
			assert.Equal(t, int32(30), result.Int32())

			jsMultiplyMethod := createAndRegisterJSFunc(t, ctx, "calcMultiply", calc.Multiply)
			defer jsMultiplyMethod.Free()
			result2 := must(ctx.Eval("test.js", qjs.Code("calcMultiply(3.5, 2.0)")))
			defer result2.Free()
			assert.Equal(t, float64(7), result2.Float64())

			jsSumMethod := createAndRegisterJSFunc(t, ctx, "calcSum", calc.Sum)
			defer jsSumMethod.Free()
			result3 := must(ctx.Eval("test.js", qjs.Code("calcSum(1, 2, 3)")))
			defer result3.Free()
			assert.Equal(t, int32(6), result3.Int32())
		})

		t.Run("PointerReceiverMethods", func(t *testing.T) {
			calc := &Calculator{Name: "TestCalc"}

			jsSetNameMethod := createAndRegisterJSFunc(t, ctx, "setName", calc.SetName)
			defer jsSetNameMethod.Free()

			result := must(ctx.Eval("test.js", qjs.Code(`setName("NewName")`)))
			defer result.Free()
			assert.True(t, result.IsUndefined() || result.IsNull())
			assert.Equal(t, "NewName", calc.Name)

			_, err := ctx.Eval("test.js", qjs.Code(`setName("")`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "name cannot be empty")
		})
	})
}

func TestFuncToJSErrorHandling(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("ArgumentHandling", func(t *testing.T) {
		t.Run("ArgumentCountMismatch", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "manyParamFunc", func(a, b, c, d, e int) int {
				return a + b + c + d + e
			})

			result := must(ctx.Eval("test.js", qjs.Code(`manyParamFunc(1, 2, 3, 4, 5)`)))
			defer result.Free()
			assert.Equal(t, int32(15), result.Int32())

			createAndRegisterJSFunc(t, ctx, "paramFunc", func(a, b, c int) int {
				return a + b + c
			})

			result2 := must(ctx.Eval("test.js", qjs.Code(`paramFunc(10)`)))
			defer result2.Free()
			assert.Equal(t, int32(10), result2.Int32())

			createAndRegisterJSFunc(t, ctx, "paramFunc2", func(a int) int {
				return a * 2
			})

			result3 := must(ctx.Eval("test.js", qjs.Code(`paramFunc2(5, 10, 15)`)))
			defer result3.Free()
			assert.Equal(t, int32(10), result3.Int32())
		})

		t.Run("ArgumentConversionWithCleanup", func(t *testing.T) {
			createAndRegisterJSFunc(t, ctx, "funcWithError", func(a, b, c int) string {
				return fmt.Sprintf("%d %d %d", a, b, c)
			})

			// Test with too many arguments to verify truncation works
			result := must(ctx.Eval("test.js", qjs.Code(`funcWithError(1, 2, 3, 4, 5)`)))
			defer result.Free()
			assert.Equal(t, "1 2 3", result.String())
		})
	})

	t.Run("ReturnValueValidation", func(t *testing.T) {
		_, err := qjs.FuncToJS(ctx, func(a int) (int, string, error) {
			return a, "test", nil
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 0-2")

		_, err2 := qjs.FuncToJS(ctx, func(a int) (int, string) {
			return a, "test"
		})
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "expected second return to be error")
	})

	t.Run("InvalidTypes", func(t *testing.T) {
		// Not a function
		_, err := qjs.FuncToJS(ctx, "not a function")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected GO target function")
	})
}
