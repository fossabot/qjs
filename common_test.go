package qjs_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
)

func TestParseTimezone(t *testing.T) {
	t.Run("IANA_timezone_names", func(t *testing.T) {
		// Test valid IANA timezone names
		testCases := []struct {
			name     string
			timezone string
			expected string
		}{
			{"UTC", "UTC", "UTC"},
			{"America_New_York", "America/New_York", "America/New_York"},
			{"Asia_Tokyo", "Asia/Tokyo", "Asia/Tokyo"},
			{"Europe_London", "Europe/London", "Europe/London"},
			{"Australia_Sydney", "Australia/Sydney", "Australia/Sydney"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected, result.String())
			})
		}
	})

	t.Run("UTC_offset_formats", func(t *testing.T) {
		testCases := []struct {
			name           string
			timezone       string
			expectedName   string
			expectedOffset int // in seconds
		}{
			// 5-character format with colon (+HH:MM)
			{"positive_offset_colon_format", "+05:30", "+05:30", 5*3600 + 30*60},
			{"negative_offset_colon_format", "-08:00", "-08:00", -(8 * 3600)},
			{"zero_offset_colon_format", "+00:00", "+00:00", 0},
			{"max_positive_colon", "+23:59", "+23:59", 23*3600 + 59*60},
			{"max_negative_colon", "-23:59", "-23:59", -(23*3600 + 59*60)},

			// 4-character format without colon (+HHMM)
			{"positive_offset_no_colon", "+0530", "+0530", 5*3600 + 30*60},
			{"negative_offset_no_colon", "-0800", "-0800", -(8 * 3600)},
			{"zero_offset_no_colon", "+0000", "+0000", 0},
			{"max_positive_no_colon", "+2359", "+2359", 23*3600 + 59*60},
			{"max_negative_no_colon", "-2359", "-2359", -(23*3600 + 59*60)},

			// 2-character format hours only (+HH)
			{"positive_hours_only", "+05", "+05", 5 * 3600},
			{"negative_hours_only", "-08", "-08", -(8 * 3600)},
			{"zero_hours_only", "+00", "+00", 0},
			{"max_positive_hours", "+23", "+23", 23 * 3600},
			{"max_negative_hours", "-23", "-23", -(23 * 3600)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectedName, result.String())

				// Test offset by checking time conversion
				testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
				convertedTime := testTime.In(result)
				_, offset := convertedTime.Zone()
				assert.Equal(t, tc.expectedOffset, offset)
			})
		}
	})

	t.Run("unusual_but_valid_offset_formats", func(t *testing.T) {
		testCases := []struct {
			name     string
			timezone string
			expected string
		}{
			{"extra_characters_after_valid", "+05:30extra", "+05:30extra"},
			{"letters_that_parse_to_zero", "+ab:cd", "+ab:cd"}, // parses as 0:0
			{"incomplete_colon_format", "+05:", "+05:"},        // parses as 5:0
			{"invalid_colon_position", "+0:530", "+0:530"},     // parses as 0:530, but 530 > 59 so might be invalid
			{"three_digit_format", "+123", "+123"},             // treated as unknown format, parses as 0:0
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected, result.String())
			})
		}
	})

	t.Run("invalid_UTC_offset_formats_fallback_to_UTC", func(t *testing.T) {
		// These are the actual conditions that cause fallback to UTC:
		// 1. Length < 3
		// 2. Doesn't start with + or -
		// 3. Hours > 23 or Minutes > 59 (after parsing)
		testCases := []struct {
			name     string
			timezone string
			reason   string
		}{
			// Length < 3 conditions
			{"too_short_with_sign", "+1", "length < 3"},
			{"too_short_with_minus", "-", "length < 3"},

			// No + or - prefix
			{"no_sign_prefix", "0530", "doesn't start with + or -"},
			{"invalid_sign", "*0530", "invalid sign character"},

			// These should actually fallback due to validation failure
			{"invalid_minutes_over_59_colon", "+05:60", "minutes > 59"},
			{"invalid_minutes_over_59_no_colon", "+0560", "minutes > 59"},
			{"invalid_hours_over_23_colon", "+24:00", "hours > 23"},
			{"invalid_hours_over_23_no_colon", "+2400", "hours > 23"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, "UTC", result.String(), "Should fallback to UTC for: %s", tc.reason)
			})
		}
	})

	t.Run("invalid_IANA_timezone_fallback_to_UTC", func(t *testing.T) {
		testCases := []string{
			"Invalid/Timezone",
			"NonExistent/Location",
			"Bad_Format",
			"America/NonExistentCity",
			"",
			"Random_String",
		}

		for _, tz := range testCases {
			t.Run("invalid_"+tz, func(t *testing.T) {
				result := qjs.ParseTimezone(tz)
				assert.NotNil(t, result)
				assert.Equal(t, "UTC", result.String(), "Should fallback to UTC for invalid IANA timezone: %s", tz)
			})
		}
	})

	t.Run("edge_cases", func(t *testing.T) {
		testCases := []struct {
			name     string
			timezone string
			expected string
		}{
			{"empty_string", "", "UTC"},
			{"just_plus", "+", "UTC"},
			{"just_minus", "-", "UTC"},
			{"single_digit", "5", "UTC"},
			{"whitespace", "   ", "UTC"},
			{"plus_zero", "+0", "UTC"},  // 1 character after +, should fallback
			{"minus_zero", "-0", "UTC"}, // 1 character after -, should fallback
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected, result.String())
			})
		}
	})

	t.Run("boundary_validation", func(t *testing.T) {
		// Test exact boundary conditions for hours and minutes validation
		testCases := []struct {
			name        string
			timezone    string
			shouldBeUTC bool
		}{
			// Valid boundaries
			{"min_valid_hour", "+00:00", false},
			{"max_valid_hour", "+23:00", false},
			{"min_valid_minute", "+12:00", false},
			{"max_valid_minute", "+12:59", false},

			// Invalid boundaries that should fallback to UTC
			{"hour_exactly_24", "+24:00", true},
			{"minute_exactly_60", "+12:60", true},
			{"hour_over_24", "+25:00", true},
			{"minute_over_60", "+12:61", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				if tc.shouldBeUTC {
					assert.Equal(t, "UTC", result.String())
				} else {
					assert.NotEqual(t, "UTC", result.String())
				}
			})
		}
	})

	t.Run("sign_handling", func(t *testing.T) {
		testCases := []struct {
			name       string
			timezone   string
			expectSign int
		}{
			{"positive_sign", "+05:00", 1},
			{"negative_sign", "-05:00", -1},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)

				// Check the actual offset sign
				testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
				convertedTime := testTime.In(result)
				_, offset := convertedTime.Zone()

				if tc.expectSign > 0 {
					assert.True(t, offset >= 0, "Expected positive or zero offset")
				} else {
					assert.True(t, offset < 0, "Expected negative offset")
				}
			})
		}
	})

	t.Run("format_length_branches", func(t *testing.T) {
		testCases := []struct {
			name     string
			timezone string
			expected string
		}{
			// 5-character with colon (len(offset) == 5 && offset[2] == ':')
			{"five_char_with_colon", "+12:34", "+12:34"},

			// 4-character without colon (len(offset) == 4)
			{"four_char_no_colon", "+1234", "+1234"},

			// 2-character hours only (len(offset) == 2)
			{"two_char_hours", "+12", "+12"},

			// Other lengths fall through with zero values
			{"one_char_after_sign", "+1", "UTC"},      // length < 3 total, should be UTC
			{"three_char_no_colon", "+123", "+123"},   // doesn't match any specific format, gets 0:0
			{"six_char_format", "+12:345", "+12:345"}, // doesn't match format, gets whatever fmt.Sscanf parses
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.ParseTimezone(tc.timezone)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected, result.String())
			})
		}
	})
}

// Test struct types for FieldMapper concurrency testing
type ConcurrencyTestStruct struct {
	Name   string
	Value  int
	Active bool
}

type ComplexConcurrencyStruct struct {
	ID          int            `json:"id"`
	Data        map[string]any `json:"data"`
	Timestamp   time.Time      `json:"timestamp"`
	PrivateData string         `json:"-"`
	Nested      struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	} `json:"nested"`
}

type EmbeddedConcurrencyStruct struct {
	ConcurrencyTestStruct
	ExtraField string
}

// TestFieldMapperDoubleCheckLocking tests the double-check locking pattern.
func TestFieldMapperDoubleCheckLocking(t *testing.T) {
	const numAttempts = 100
	for attempt := 0; attempt < numAttempts; attempt++ {
		fm := qjs.NewFieldMapper()
		structType := reflect.TypeOf(ConcurrencyTestStruct{})

		const numGoroutines = 200 // Very high contention
		var wg sync.WaitGroup
		var startBarrier sync.WaitGroup
		var results sync.Map

		startBarrier.Add(1)
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				// All goroutines wait at the barrier
				startBarrier.Wait()

				// Immediate call to GetFieldMap to maximize race condition
				fieldMap := fm.GetFieldMap(structType)

				// Store result for verification
				results.Store(id, len(fieldMap))
			}(i)
		}

		// Release all goroutines at exactly the same time
		startBarrier.Done()
		wg.Wait()

		// Verify all results are consistent
		var count int
		results.Range(func(key, value any) bool {
			count++
			assert.Equal(t, 3, value.(int), "All goroutines should get same field count")
			return true
		})

		assert.Equal(t, numGoroutines, count, "Should have results from all goroutines")
	}
}

func TestJsNumericToGoConverterErrorHandling(t *testing.T) {
	testCases := []struct {
		name       string
		targetType reflect.Type
		floatVal   float64
		expectErr  bool
		errMsg     string
	}{
		// Valid numeric types (should not error)
		{
			name:       "valid_int_type",
			targetType: reflect.TypeOf(int(0)),
			floatVal:   42.0,
			expectErr:  false,
		},
		{
			name:       "valid_float64_type",
			targetType: reflect.TypeOf(float64(0)),
			floatVal:   3.14,
			expectErr:  false,
		},

		// Invalid types that will trigger FloatToInt error
		{
			name:       "string_type_error",
			targetType: reflect.TypeOf(""),
			floatVal:   42.0,
			expectErr:  true,
			errMsg:     "unsupported numeric type: string",
		},
		{
			name:       "bool_type_error",
			targetType: reflect.TypeOf(true),
			floatVal:   1.0,
			expectErr:  true,
			errMsg:     "unsupported numeric type: bool",
		},
		{
			name:       "slice_type_error",
			targetType: reflect.TypeOf([]int{}),
			floatVal:   42.0,
			expectErr:  true,
			errMsg:     "unsupported numeric type: slice",
		},
		{
			name:       "map_type_error",
			targetType: reflect.TypeOf(map[string]int{}),
			floatVal:   42.0,
			expectErr:  true,
			errMsg:     "unsupported numeric type: map",
		},
		{
			name:       "struct_type_error",
			targetType: reflect.TypeOf(struct{}{}),
			floatVal:   42.0,
			expectErr:  true,
			errMsg:     "unsupported numeric type: struct",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			converter := qjs.NewJsNumericToGoConverter(tc.targetType)
			result, err := converter.Convert(tc.floatVal)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestJsArrayToGoConverterErrorWrapping tests that errors are properly wrapped with newJsToGoErr
func TestJsArrayToGoConverterErrorWrapping(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	// Create a non-array value
	jsValue, err := rt.Eval("test.js", qjs.Code("123"))
	assert.NoError(t, err)
	defer jsValue.Free()

	converter := qjs.NewJsArrayToGoConverter[[]int](jsValue)
	_, err = converter.Convert()

	assert.Error(t, err)

	// Check error wrapping structure from newJsToGoErr
	assert.Contains(t, err.Error(), "cannot convert JS")
	assert.Contains(t, err.Error(), "Array")
	assert.Contains(t, err.Error(), "expected JS array, got Number=123")
}

// TestJsArrayToGoConverterJSONStringifyError tests the JSONStringify error.
func TestJsArrayToGoConverterJSONStringifyError(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	// Create a JS array with circular reference that will fail JSONStringify
	jsValue, err := rt.Eval("test.js", qjs.Code(`
		const obj = {};
		obj.self = obj; // circular reference
		[obj]  // array containing circular reference object
	`))
	assert.NoError(t, err)
	defer jsValue.Free()

	// CustomUnmarshaler is a struct type (not slice/array) so it will trigger convertViaJSON
	type TestStruct struct {
		Value string
	}

	// This should trigger the JSONStringify error path in convertViaJSON
	converter := qjs.NewJsArrayToGoConverter[TestStruct](jsValue)
	_, err = converter.Convert()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "js array:")
}

// TestFloatToIntUintptrConversion tests the specific uintptr case.
func TestFloatToIntUintptrConversion(t *testing.T) {
	testCases := []struct {
		name        string
		floatVal    float64
		targetKind  reflect.Kind
		expected    any
		expectError bool
	}{
		{
			name:       "uintptr_conversion_positive",
			floatVal:   42.0,
			targetKind: reflect.Uintptr,
			expected:   uintptr(42),
		},
		{
			name:       "uintptr_conversion_zero",
			floatVal:   0.0,
			targetKind: reflect.Uintptr,
			expected:   uintptr(0),
		},
		{
			name:       "uintptr_conversion_large_value",
			floatVal:   0xdeadbeef,
			targetKind: reflect.Uintptr,
			expected:   uintptr(0xdeadbeef),
		},
		{
			name:        "unsupported_type",
			floatVal:    42.0,
			targetKind:  reflect.String,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := qjs.FloatToInt(tc.floatVal, tc.targetKind)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported numeric type")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)

				// Verify the type is correct
				assert.Equal(t, tc.targetKind, reflect.TypeOf(result).Kind())
			}
		})
	}
}

func TestStringToNumeric(t *testing.T) {
	t.Run("empty_string_error", func(t *testing.T) {
		result, err := qjs.StringToNumeric("", reflect.TypeOf(int(0)))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "empty string cannot be converted to number")
	})

	t.Run("pointer_type_error_propagation", func(t *testing.T) {
		ptrType := reflect.TypeOf((*int)(nil))
		result, err := qjs.StringToNumeric("invalid_number", ptrType)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot convert JS string")
	})

	t.Run("unsupported_type_error", func(t *testing.T) {
		result, err := qjs.StringToNumeric("42", reflect.TypeOf("string"))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot convert JS string")
		assert.Contains(t, err.Error(), "string")
	})

	t.Run("invalid_string_parsing_errors", func(t *testing.T) {
		// Test invalid strings that can't be parsed as numbers
		invalidStrings := []string{
			"not_a_number",
			"42abc",
			"3.14.15",
			"--42",
			"++42",
			"4.2e",
			"abc123",
			"12.34.56",
		}

		targetTypes := []reflect.Type{
			reflect.TypeOf(int(0)),
			reflect.TypeOf(float32(0)),
			reflect.TypeOf(float64(0)),
		}

		for _, invalidStr := range invalidStrings {
			for _, targetType := range targetTypes {
				t.Run(fmt.Sprintf("invalid_%s_to_%s", invalidStr, targetType.Kind()), func(t *testing.T) {
					result, err := qjs.StringToNumeric(invalidStr, targetType)
					assert.Error(t, err)
					assert.Nil(t, result)
					assert.Contains(t, err.Error(), "cannot convert JS string")
				})
			}
		}
	})
}

// TestAnyToError tests the AnyToError function for panic recovery.
func TestAnyToError(t *testing.T) {
	t.Run("nil_input_returns_nil", func(t *testing.T) {
		result := qjs.AnyToError(nil)
		assert.Nil(t, result)
	})

	t.Run("error_input_returns_same_error", func(t *testing.T) {
		originalErr := fmt.Errorf("original error message")
		result := qjs.AnyToError(originalErr)

		assert.NotNil(t, result)
		assert.Equal(t, originalErr, result)
		assert.Same(t, originalErr, result) // Should be the exact same error instance
	})

	t.Run("other_input_wrapped_with_panic_prefix", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    any
			expected string
		}{
			{
				name:     "simple_string",
				input:    "panic message",
				expected: "recovered from panic: panic message",
			},
			{
				name:     "multiline_string",
				input:    "line1\nline2\nline3",
				expected: "recovered from panic: line1\nline2\nline3",
			},
			{
				name:     "integer_value",
				input:    42,
				expected: "recovered from panic: 42",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := qjs.AnyToError(tc.input)

				assert.NotNil(t, result)
				assert.Equal(t, tc.expected, result.Error())
			})
		}
	})
}
