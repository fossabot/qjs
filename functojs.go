package qjs

import (
	"fmt"
	"reflect"
)

// FuncToJS converts a Go function to a JavaScript function.
func FuncToJS(c *Context, v any) (_ *Value, err error) {
	if v == nil {
		return c.NewNull(), nil
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("[FuncToJS] %w", err)
		}
	}()

	rval := reflect.ValueOf(v)
	rtype := reflect.TypeOf(v)

	if rtype.Kind() == reflect.Ptr {
		if rval.IsNil() {
			return c.NewNull(), nil
		}

		rval = rval.Elem()
		rtype = rtype.Elem()
	}

	if rtype.Kind() != reflect.Func {
		return nil, newInvalidGoTargetErr("function", v)
	}

	if rval.IsNil() {
		return c.NewNull(), nil
	}

	fnType := rval.Type()
	if err := VerifyGoFunc(fnType); err != nil {
		return nil, err
	}

	return c.Function(func(this *This) (*Value, error) {
		goArgs, err := JsFuncArgsToGo(this.Args(), fnType)
		if err != nil {
			return nil, err
		}

		var results []reflect.Value
		if fnType.IsVariadic() {
			results = rval.CallSlice(goArgs)
		} else {
			results = rval.Call(goArgs)
		}

		return GoFuncResultToJs(c, results)
	}), nil
}

// VerifyGoFunc validates that a function signature is compatible with JS conversion.
// Ensures functions have 0-2 return values, with optional error as second return.
func VerifyGoFunc(fnType reflect.Type) error {
	signature := CreateGoFuncSignature(fnType)
	numOut := fnType.NumOut()

	if numOut > MaxGoFuncReturnValues {
		return fmt.Errorf("expected 0-2 return values, got '%s'", signature)
	}

	if numOut == MaxGoFuncReturnValues && !IsImplementError(fnType.Out(1)) {
		return fmt.Errorf("expected second return to be error, got '%s'", signature)
	}

	if numOut >= 1 {
		err := IsConvertibleToJs(fnType.Out(0), make(map[reflect.Type]bool), "func return")
		if err != nil {
			return err
		}
	}

	for i := range fnType.NumIn() {
		err := IsConvertibleToJs(fnType.In(i), make(map[reflect.Type]bool), "func param")
		if err != nil {
			return fmt.Errorf("parameter %d error: %w", i, err)
		}
	}

	return nil
}

// JsFuncArgsToGo converts JS arguments to Go arguments in both variadic and non-variadic functions,
// filling missing arguments with zero values.
func JsFuncArgsToGo(jsArgs []*Value, fnType reflect.Type) ([]reflect.Value, error) {
	numGoArgs := fnType.NumIn()
	goArgs := make([]reflect.Value, 0, numGoArgs)
	numJSArgs := len(jsArgs)

	if fnType.IsVariadic() {
		fixedArgs := Min(numGoArgs-1, numJSArgs)
		for i := range fixedArgs {
			goVal, err := JsArgToGo(jsArgs[i], fnType.In(i))
			if err != nil {
				return nil, newArgConversionErr(i, err)
			}

			goArgs = append(goArgs, goVal)
		}

		// Handle variadic slice
		variadicSlice, err := CreateVariadicSlice(jsArgs[fixedArgs:], fnType.In(numGoArgs-1), fixedArgs)
		if err != nil {
			return nil, err
		}

		return append(goArgs, variadicSlice), nil
	}

	// Limit JS args to Go args count to avoid index out of bounds
	argsToProcess := Min(numJSArgs, numGoArgs)
	for i := range argsToProcess {
		goVal, err := JsArgToGo(jsArgs[i], fnType.In(i))
		if err != nil {
			return nil, newArgConversionErr(i, err)
		}

		goArgs = append(goArgs, goVal)
	}

	// Fill missing arguments with zero values
	for i := argsToProcess; i < numGoArgs; i++ {
		goArgs = append(goArgs, reflect.Zero(fnType.In(i)))
	}

	return goArgs, nil
}

// handlePointerArgument processes JS arguments for pointer types.
func handlePointerArgument(jsArg *Value, argType reflect.Type) (reflect.Value, error) {
	if jsArg.IsNull() || jsArg.IsUndefined() {
		return reflect.Zero(argType), nil
	}

	underlyingType := argType.Elem()
	zeroVal := reflect.New(underlyingType).Elem()

	goVal, err := JsValueToGo(jsArg, zeroVal.Interface())
	if err != nil {
		return reflect.Value{}, newJsToGoErr(jsArg, err, "function param pointer")
	}

	ptrVal := reflect.New(underlyingType)
	ptrVal.Elem().Set(reflect.ValueOf(goVal))

	return ptrVal, nil
}

// JsArgToGo converts a single JS argument to a Go value with enhanced type handling.
func JsArgToGo(jsArg *Value, argType reflect.Type) (reflect.Value, error) {
	if argType.Kind() == reflect.Ptr {
		return handlePointerArgument(jsArg, argType)
	}

	goZeroVal := reflect.Zero(argType).Interface()

	goVal, err := JsValueToGo(jsArg, goZeroVal)
	if err != nil {
		return reflect.Value{}, newJsToGoErr(jsArg, err, "function param")
	}

	return reflect.ValueOf(goVal), nil
}

// CreateVariadicSlice creates a reflect.Value slice for variadic arguments.
// Converts remaining JS arguments to the slice element type and returns as a slice value.
func CreateVariadicSlice(jsArgs []*Value, sliceType reflect.Type, fixedArgsCount int) (reflect.Value, error) {
	varArgType := sliceType.Elem()
	numVarArgs := len(jsArgs)
	variadicSlice := reflect.MakeSlice(sliceType, numVarArgs, numVarArgs)

	for i, jsArg := range jsArgs {
		goVal, err := JsArgToGo(jsArg, varArgType)
		if err != nil {
			return reflect.Value{}, newArgConversionErr(fixedArgsCount+i, err)
		}

		variadicSlice.Index(i).Set(goVal)
	}

	return variadicSlice, nil
}

// GoFuncResultToJs processes Go function call results and converts them to JS values.
// Supports error-only, single value, and (value, error) return patterns.
// Errors are thrown in JS context, other values are converted to appropriate JS types.
func GoFuncResultToJs(c *Context, results []reflect.Value) (*Value, error) {
	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		result := results[0]
		if IsImplementError(result.Type()) {
			if !result.IsNil() {
				resultErr, _ := result.Interface().(error)
				c.ThrowError(resultErr)

				return nil, nil
			}

			return nil, nil
		}

		return ToJSValue(c, result.Interface())
	}

	// Handle (value, error) return pattern
	if len(results) == MaxGoFuncReturnValues {
		lastResult := results[1]
		if IsImplementError(lastResult.Type()) {
			if !lastResult.IsNil() {
				resultErr, _ := lastResult.Interface().(error)
				c.ThrowError(resultErr)

				return nil, nil
			}

			return ToJSValue(c, results[0].Interface())
		}
	}

	return ToJSValue(c, results[0].Interface())
}
