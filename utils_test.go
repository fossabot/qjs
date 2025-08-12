package qjs_test

import (
	"reflect"
	"testing"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
)

func TestIsConvertibleToJs(t *testing.T) {
	t.Run("BasicTypes", func(t *testing.T) {
		basicTypes := []struct {
			name     string
			value    any
			expected bool
		}{
			{"int", 42, true},
			{"string", "hello", true},
			{"bool", true, true},
			{"float64", 3.14, true},
			{"[]int", []int{1, 2, 3}, true},
			{"map[string]int", map[string]int{"key": 1}, true},
		}

		for _, tt := range basicTypes {
			t.Run(tt.name, func(t *testing.T) {
				err := qjs.IsConvertibleToJs(reflect.TypeOf(tt.value), make(map[reflect.Type]bool), "test")
				if tt.expected {
					assert.NoError(t, err, "Expected %s to be convertible", tt.name)
				} else {
					assert.Error(t, err, "Expected %s to not be convertible", tt.name)
				}
			})
		}
	})

	t.Run("UnsupportedTypes", func(t *testing.T) {
		t.Run("channel", func(t *testing.T) {
			err := qjs.IsConvertibleToJs(reflect.TypeOf(make(chan int)), make(map[reflect.Type]bool), "test")
			assert.Error(t, err, "Channel should not be convertible")
			assert.Contains(t, err.Error(), "channel", "Error should mention channel")
		})
	})

	t.Run("StructWithFields", func(t *testing.T) {
		t.Run("ExportedFields", func(t *testing.T) {
			type TestStruct struct {
				PublicField  string
				AnotherField int
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct with exported fields should be convertible")
		})

		t.Run("UnexportedFieldsIgnored", func(t *testing.T) {
			type TestStruct struct {
				PublicField  string
				privateField string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct with unexported fields should be convertible (unexported fields ignored)")
		})

		t.Run("MixedFieldsWithUnsupportedPrivate", func(t *testing.T) {
			type TestStruct struct {
				PublicField  string
				privateField chan int
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct should be convertible when unsupported types are in unexported fields")
		})
	})

	t.Run("StructWithJSONTags", func(t *testing.T) {
		t.Run("JSONOmitTag", func(t *testing.T) {
			type TestStruct struct {
				PublicField  string
				OmittedField chan int `json:"-"`
				AnotherField int
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct should be convertible when unsupported types have json:\"-\" tag")
		})

		t.Run("JSONRenameTag", func(t *testing.T) {
			type TestStruct struct {
				Field            string   `json:"renamed_field"`
				UnsupportedField chan int `json:"-"`
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct should be convertible with JSON rename tags")
		})

		t.Run("UnsupportedFieldWithoutOmitTag", func(t *testing.T) {
			type TestStruct struct {
				PublicField      string
				UnsupportedField chan int
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.Error(t, err, "Struct should not be convertible with exported unsupported fields")
			assert.Contains(t, err.Error(), "TestStruct.UnsupportedField", "Error should mention the problematic field")
		})

		t.Run("ComplexJSONTags", func(t *testing.T) {
			type TestStruct struct {
				Field1         string    `json:"field1,omitempty"`
				Field2         int       `json:"field2"`
				OmittedField   chan int  `json:"-"`
				AnotherOmitted chan bool `json:"-,"`
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(TestStruct{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct should handle complex JSON tags correctly")
		})
	})

	t.Run("RecursiveTypes", func(t *testing.T) {
		type RecursiveStruct struct {
			Name string
			Self *RecursiveStruct
		}

		err := qjs.IsConvertibleToJs(reflect.TypeOf(RecursiveStruct{}), make(map[reflect.Type]bool), "test")
		assert.NoError(t, err, "Recursive struct should be convertible")
	})

	t.Run("NestedStructs", func(t *testing.T) {
		t.Run("ValidNested", func(t *testing.T) {
			type Inner struct {
				Value int
			}
			type Outer struct {
				Inner Inner
				Name  string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Nested convertible structs should be convertible")
		})

		t.Run("InvalidNested", func(t *testing.T) {
			type Inner struct {
				BadField chan int
			}
			type Outer struct {
				Inner Inner
				Name  string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.Error(t, err, "Nested struct with unsupported fields should not be convertible")
		})

		t.Run("NestedWithOmittedField", func(t *testing.T) {
			type Inner struct {
				GoodField string
				BadField  chan int `json:"-"`
			}
			type Outer struct {
				Inner Inner
				Name  string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Nested struct with omitted unsupported fields should be convertible")
		})

		t.Run("EmbeddedPointerStruct", func(t *testing.T) {
			type Inner struct {
				GoodField string
			}
			type Outer struct {
				*Inner
				Name string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct with embedded pointer to struct should be convertible")
		})

		t.Run("EmbeddedPointerWithUnsupportedField", func(t *testing.T) {
			type Inner struct {
				BadField chan int
			}
			type Outer struct {
				*Inner
				Name string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.Error(t, err, "Struct with embedded pointer containing unsupported fields should not be convertible")
		})

		t.Run("EmbeddedPointerWithOmittedField", func(t *testing.T) {
			type Inner struct {
				GoodField string
				BadField  chan int `json:"-"`
			}
			type Outer struct {
				*Inner
				Name string
			}

			err := qjs.IsConvertibleToJs(reflect.TypeOf(Outer{}), make(map[reflect.Type]bool), "test")
			assert.NoError(t, err, "Struct with embedded pointer containing omitted unsupported fields should be convertible")
		})
	})

	t.Run("Pointers", func(t *testing.T) {
		type TestStruct struct {
			Field        string
			OmittedField chan int `json:"-"`
		}

		err := qjs.IsConvertibleToJs(reflect.TypeOf(&TestStruct{}), make(map[reflect.Type]bool), "test")
		assert.NoError(t, err, "Pointer to convertible struct should be convertible")
	})
}
