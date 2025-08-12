package qjs

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

// ToJSValue converts any Go value to a QuickJS value.
func ToJSValue(c *Context, v any) (*Value, error) {
	return NewTracker[uintptr]().ToJSValue(c, v)
}

// ToJSValue converts any Go value to a QuickJS Value using the conversion context.
func (tracker *Tracker[T]) ToJSValue(c *Context, v any) (*Value, error) {
	if v == nil {
		return c.NewNull(), nil
	}

	if err, ok := v.(error); ok {
		return c.NewError(err), nil
	}

	// Fast path for common types
	if jsVal := tryConvertBuiltinTypes(c, v); jsVal != nil {
		return jsVal, nil
	}

	// Numeric types handling - consolidated
	if jsVal := tryConvertNumeric(c, v); jsVal != nil {
		return jsVal, nil
	}

	return tracker.convertReflectValue(c, v)
}

// StructToJSObjectValue converts a Go struct to a JavaScript object.
// Includes both fields and methods as object properties.
func (tracker *Tracker[T]) StructToJSObjectValue(
	c *Context,
	rtype reflect.Type,
	rval reflect.Value,
) (*Value, error) {
	return withJSObject(c, func(obj *Value) error {
		// Determine the struct type and value for field processing:
		// - pointer to struct: dereference for field processing.
		// - direct struct: use as-is.
		var (
			structType reflect.Type
			structVal  reflect.Value
		)

		if rtype.Kind() == reflect.Ptr {
			structType = rtype.Elem()
			structVal = rval.Elem()
		} else {
			structType = rtype
			structVal = rval
		}

		err := tracker.addStructFieldsToObject(c, obj, structType, structVal)
		if err != nil {
			return err
		}

		// For methods, use the original type and value
		// to preserve pointer receiver methods
		return tracker.addStructMethodsToObject(c, obj, rtype, rval)
	})
}

// SliceToArrayValue converts a Go slice to a JavaScript array.
func (tracker *Tracker[T]) SliceToArrayValue(c *Context, rval reflect.Value) (*Value, error) {
	return tracker.arrayLikeToJS(c, rval, "slice")
}

// ArrayToArrayValue converts a Go array to a JavaScript array without unnecessary copying.
func (tracker *Tracker[T]) ArrayToArrayValue(c *Context, rval reflect.Value) (*Value, error) {
	return tracker.arrayLikeToJS(c, rval, "array")
}

// MapToObjectValue converts a Go map to a JavaScript object.
// Non-string keys are converted to string representation.
func (tracker *Tracker[T]) MapToObjectValue(
	c *Context,
	rval reflect.Value,
) (*Value, error) {
	return withJSObject(c, func(obj *Value) error {
		for _, key := range rval.MapKeys() {
			value := rval.MapIndex(key)

			var keyStr string
			if key.Kind() == reflect.String {
				keyStr = key.String()
			} else {
				keyStr = fmt.Sprintf("%v", key.Interface())
			}

			jsValue, err := tracker.ToJSValue(c, value.Interface())
			if err != nil {
				return newGoToJsErr("map key: "+keyStr, err)
			}

			obj.SetPropertyStr(keyStr, jsValue)
		}

		return nil
	})
}

// GoNumberToJS converts Go numeric types to appropriate JS number types.
func GoNumberToJS[T NumberType](c *Context, i T) *Value {
	switch v := any(i).(type) {
	case int32:
		return c.NewInt32(v)
	case uint32:
		return c.NewUint32(v)
	case int64:
		return c.NewInt64(v)
	case float64:
		return c.NewFloat64(v)
	case float32:
		return c.NewFloat64(float64(v))
	default:
		// Convert other integer types to int64, floats to float64
		switch any(i).(type) {
		case int, int8, int16:
			return c.NewInt64(int64(i))
		case uint, uint8, uint16, uint64:
			if uint64(i) > math.MaxInt64 {
				return c.NewBigUint64(uint64(i))
			}

			return c.NewInt64(int64(i))
		default:
			return c.NewFloat64(float64(i))
		}
	}
}

// GoComplexToJS converts Go complex numbers to JS objects with real/imag properties.
func GoComplexToJS[T complex64 | complex128](c *Context, z T) *Value {
	obj := c.NewObject()

	var realPart, imagPart float64

	switch v := any(z).(type) {
	case complex64:
		realPart = float64(real(v))
		imagPart = float64(imag(v))
	case complex128:
		realPart = real(v)
		imagPart = imag(v)
	}

	obj.SetPropertyStr("real", c.NewFloat64(realPart))
	obj.SetPropertyStr("imag", c.NewFloat64(imagPart))

	return obj
}

// Backward compatibility functions for the old API

// StructToJSObjectValue provides backward compatibility for the old API.
func StructToJSObjectValue(c *Context, rtype reflect.Type, rval reflect.Value) (*Value, error) {
	ctx := NewTracker[uint64]()

	return ctx.StructToJSObjectValue(c, rtype, rval)
}

// SliceToArrayValue provides backward compatibility for the old API.
func SliceToArrayValue(c *Context, rval reflect.Value) (*Value, error) {
	ctx := NewTracker[uint64]()

	return ctx.SliceToArrayValue(c, rval)
}

// MapToObjectValue provides backward compatibility for the old API.
func MapToObjectValue(c *Context, rval reflect.Value) (*Value, error) {
	ctx := NewTracker[uint64]()

	return ctx.MapToObjectValue(c, rval)
}

// tryConvertBuiltinTypes handles built-in Go types that don't require reflection.
func tryConvertBuiltinTypes(c *Context, v any) *Value {
	switch val := v.(type) {
	case bool:
		return c.NewBool(val)
	case string:
		return c.NewString(val)
	case []byte:
		if val == nil {
			return c.NewNull()
		}

		return c.NewArrayBuffer(val)
	case time.Time:
		return c.NewDate(&val)
	case *time.Time:
		if val == nil {
			return c.NewNull()
		}

		return c.NewDate(val)
	case uintptr:
		return c.NewInt64(int64(val))
	}

	return nil
}

// tryConvertNumeric handles all numeric types in a consolidated way.
func tryConvertNumeric(c *Context, v any) *Value {
	switch val := v.(type) {
	case int8:
		return GoNumberToJS(c, val)
	case int16:
		return GoNumberToJS(c, val)
	case int32:
		return GoNumberToJS(c, val)
	case int:
		return GoNumberToJS(c, val)
	case int64:
		return GoNumberToJS(c, val)
	case uint8:
		return GoNumberToJS(c, val)
	case uint16:
		return GoNumberToJS(c, val)
	case uint32:
		return GoNumberToJS(c, val)
	case uint:
		return GoNumberToJS(c, val)
	case uint64:
		// Large uint64 values use float64 to avoid overflow
		if val&(uint64(1)<<Uint64SignBitPosition) == 0 {
			return GoNumberToJS(c, int64(val))
		}

		return c.NewFloat64(float64(val))
	case float32:
		return GoNumberToJS(c, val)
	case float64:
		return GoNumberToJS(c, val)
	case complex64:
		return GoComplexToJS(c, val)
	case complex128:
		return GoComplexToJS(c, val)
	}

	return nil
}

// convertReflectValue handles types that require reflection.
func (tracker *Tracker[T]) convertReflectValue(c *Context, v any) (*Value, error) {
	rtype := reflect.TypeOf(v)
	rval := reflect.ValueOf(v)

	var ct CircularTracker[T]
	defer ct.cleanup()

	if rtype.Kind() == reflect.Ptr {
		if rval.IsNil() {
			return c.NewNull(), nil
		}

		// Track pointer address for circular reference detection.
		var addr any = (rval.Pointer())

		addrT, _ := addr.(T)
		if err := ct.trackPtr(tracker, addrT); err != nil {
			return nil, newGoToJsErr(GetGoTypeName(reflect.TypeOf(v)), err, "recursive pointer")
		}

		// If pointer points to a struct,
		// preserve the pointer type for method resolution.
		elemType := rtype.Elem()
		if elemType.Kind() == reflect.Struct {
			return tracker.StructToJSObjectValue(c, rtype, rval)
		}

		return tracker.ToJSValue(c, rval.Elem().Interface())
	}

	switch rtype.Kind() {
	// Unreachable code
	// case reflect.Invalid:
	// 	return c.NewNull(), nil
	case reflect.Func:
		return FuncToJS(c, v)
	case reflect.Struct:
		// Unreachable code
		// Circular reference check for addressable structs
		// if rval.CanAddr() {
		// 	var addr any = rval.UnsafeAddr()
		// 	if err := tracker.trackPtr(ctx, addr.(T)); err != nil {
		// 		return nil, newGoToJsErr(GetGoTypeName(reflect.TypeOf(v)), err, "recursive pointer")
		// 	}
		// }
		return tracker.StructToJSObjectValue(c, rtype, rval)
	case reflect.Slice:
		if rval.IsNil() {
			return c.NewNull(), nil
		}

		return tracker.SliceToArrayValue(c, rval)
	case reflect.Map:
		if rval.IsNil() {
			return c.NewNull(), nil
		}

		return tracker.MapToObjectValue(c, rval)
	case reflect.Array:
		return tracker.ArrayToArrayValue(c, rval)
	// Unreachable code
	// case reflect.Interface:
	// 	if rval.IsNil() {
	// 		return c.NewNull(), nil
	// 	}

	// 	return ctx.ToJSValue(c, rval.Elem().Interface())
	case reflect.Chan:
		return nil, newGoToJsErr("channel: "+GetGoTypeName(rtype), nil)
	case reflect.UnsafePointer:
		return nil, newGoToJsErr(GetGoTypeName(rtype), nil)
	}

	return nil, newGoToJsErr(rtype.Name(), nil)
}

// withJSObject creates a JS object and ensures it's freed on error.
func withJSObject(c *Context, fn func(*Value) error) (*Value, error) {
	obj := c.NewObject()
	if err := fn(obj); err != nil {
		obj.Free()

		return nil, err
	}

	return obj, nil
}

// addStructFieldsToObject converts struct fields to JS object properties.
// Processes embedded structs first, then regular fields to allow overriding.
func (tracker *Tracker[T]) addStructFieldsToObject(
	c *Context,
	obj *Value,
	rtype reflect.Type,
	rval reflect.Value,
) error {
	// Embedded structs processed first to enable field overriding
	for i := range rtype.NumField() {
		field := rtype.Field(i)

		if !field.IsExported() {
			continue
		}

		if field.Anonymous {
			fieldValue := rval.Field(i)
			if fieldValue.Kind() == reflect.Struct {
				err := tracker.addStructFieldsToObject(c, obj, field.Type, fieldValue)
				if err != nil {
					return err
				}
			}
		}
	}

	// Regular fields can override embedded fields
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if !field.IsExported() {
			continue
		}

		if field.Anonymous {
			continue
		}

		fieldValue := rval.Field(i)
		fieldName := field.Name

		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			name, _, _ := strings.Cut(jsonTag, ",")
			if name == "-" {
				continue
			}

			if name != "" {
				fieldName = name
			}
		}

		prop, err := tracker.ToJSValue(c, fieldValue.Interface())
		if err != nil {
			return err
		}

		obj.SetPropertyStr(fieldName, prop)
	}

	return nil
}

// addStructMethodsToObject adds exported struct methods as JS functions.
func (tracker *Tracker[T]) addStructMethodsToObject(
	c *Context,
	obj *Value,
	rtype reflect.Type,
	rval reflect.Value,
) error {
	for i := range rtype.NumMethod() {
		method := rtype.Method(i)
		methodValue := rval.Method(i)

		jsMethod, err := FuncToJS(c, methodValue.Interface())
		if err != nil {
			return newGoToJsErr(
				GetGoTypeName(rtype)+"."+method.Name,
				err,
				"struct method",
			)
		}

		obj.SetPropertyStr(method.Name, jsMethod)
	}

	return nil
}

// arrayLikeToJS is a helper that converts arrays and slices to JS arrays.
func (tracker *Tracker[T]) arrayLikeToJS(
	c *Context,
	rval reflect.Value,
	typeName string,
) (*Value, error) {
	arr := c.NewArray()

	for i := range rval.Len() {
		elem := rval.Index(i)

		jsElem, err := tracker.ToJSValue(c, elem.Interface())
		if err != nil {
			arr.Free()

			return nil, newGoToJsErr(typeName+": "+GetGoTypeName(rval.Type()), err)
		}

		arr.SetPropertyIndex(int64(i), jsElem)
	}

	return arr.Value, nil
}
