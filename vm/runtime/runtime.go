package runtime

//go:generate sh -c "go run ./generate > ./helpers.go"

import (
	"fmt"
	"math"
	"reflect"
)

func Fetch(from, i interface{}) interface{} {
	v := reflect.ValueOf(from)
	kind := v.Kind()
	if kind == reflect.Invalid {
		panic(fmt.Sprintf("cannot fetch %v from %T", i, from))
	}

	// Methods can be defined on any type.
	if v.NumMethod() > 0 {
		method := v.MethodByName(i.(string))
		if method.IsValid() {
			return method.Interface()
		}
	}

	// Structs, maps, and slices can be access through a pointer or through
	// a value, when they are accessed through a pointer we don't want to
	// copy them to a value.
	if kind == reflect.Ptr {
		v = reflect.Indirect(v)
		kind = v.Kind()
	}

	switch kind {

	case reflect.Array, reflect.Slice, reflect.String:
		value := v.Index(ToInt(i))
		if value.IsValid() {
			return value.Interface()
		}

	case reflect.Map:
		value := v.MapIndex(reflect.ValueOf(i))
		if value.IsValid() {
			return value.Interface()
		} else {
			elem := reflect.TypeOf(from).Elem()
			return reflect.Zero(elem).Interface()
		}

	case reflect.Struct:
		fieldName := i.(string)
		value := v.FieldByNameFunc(func(name string) bool {
			field, _ := v.Type().FieldByName(name)
			if field.Tag.Get("expr") == fieldName {
				return true
			}
			return name == fieldName
		})
		if value.IsValid() {
			return value.Interface()
		}
	}
	panic(fmt.Sprintf("cannot fetch %v from %T", i, from))
}

type Field struct {
	Index []int
	Path  string
}

func FetchField(from interface{}, field *Field) interface{} {
	v := reflect.ValueOf(from)
	kind := v.Kind()
	if kind != reflect.Invalid {
		if kind == reflect.Ptr {
			v = reflect.Indirect(v)
			kind = v.Kind()
		}
		if kind == reflect.Struct {
			// We can use v.FieldByIndex here, but it will panic if the field
			// is not exists. And we need to recover() to generate a more
			// user-friendly error message.
			// Also, our fieldByIndex() function is slightly faster than the
			// v.FieldByIndex() function as we don't need to verify what a field
			// is a struct as we already did it on compilation step.
			value := fieldByIndex(v, field.Index)
			if value.IsValid() {
				return value.Interface()
			}
		}
	}
	panic(fmt.Sprintf("cannot get %v from %T", field.Path, from))
}

func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	if len(index) == 1 {
		return v.Field(index[0])
	}
	for i, x := range index {
		if i > 0 {
			if v.Kind() == reflect.Ptr {
				if v.IsNil() {
					return reflect.Value{}
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v
}

type Method struct {
	Index int
	Name  string
}

func FetchMethod(from interface{}, method *Method) interface{} {
	v := reflect.ValueOf(from)
	kind := v.Kind()
	if kind != reflect.Invalid {
		// Methods can be defined on any type, no need to dereference.
		method := v.Method(method.Index)
		if method.IsValid() {
			return method.Interface()
		}
	}
	panic(fmt.Sprintf("cannot fetch %v from %T", method.Name, from))
}

func Deref(i interface{}) interface{} {
	if i == nil {
		return nil
	}

	v := reflect.ValueOf(i)

	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return i
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return i
		}
		indirect := reflect.Indirect(v)
		switch indirect.Kind() {
		case reflect.Struct, reflect.Map, reflect.Array, reflect.Slice:
		default:
			v = v.Elem()
		}
	}

	if v.IsValid() {
		return v.Interface()
	}

	panic(fmt.Sprintf("cannot dereference %v", i))
}

func Slice(array, from, to interface{}) interface{} {
	v := reflect.ValueOf(array)

	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		length := v.Len()
		a, b := ToInt(from), ToInt(to)

		if b > length {
			b = length
		}
		if a > b {
			a = b
		}

		value := v.Slice(a, b)
		if value.IsValid() {
			return value.Interface()
		}

	case reflect.Ptr:
		value := v.Elem()
		if value.IsValid() {
			return Slice(value.Interface(), from, to)
		}

	}
	panic(fmt.Sprintf("cannot slice %v", from))
}

func In(needle interface{}, array interface{}) bool {
	if array == nil {
		return false
	}
	v := reflect.ValueOf(array)

	switch v.Kind() {

	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			value := v.Index(i)
			if value.IsValid() {
				if Equal(value.Interface(), needle).(bool) {
					return true
				}
			}
		}
		return false

	case reflect.Map:
		n := reflect.ValueOf(needle)
		if !n.IsValid() {
			panic(fmt.Sprintf("cannot use %T as index to %T", needle, array))
		}
		value := v.MapIndex(n)
		if value.IsValid() {
			return true
		}
		return false

	case reflect.Struct:
		n := reflect.ValueOf(needle)
		if !n.IsValid() || n.Kind() != reflect.String {
			panic(fmt.Sprintf("cannot use %T as field name of %T", needle, array))
		}
		value := v.FieldByName(n.String())
		if value.IsValid() {
			return true
		}
		return false

	case reflect.Ptr:
		value := v.Elem()
		if value.IsValid() {
			return In(needle, value.Interface())
		}
		return false
	}

	panic(fmt.Sprintf(`operator "in"" not defined on %T`, array))
}

func Length(a interface{}) int {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return v.Len()
	default:
		panic(fmt.Sprintf("invalid argument for len (type %T)", a))
	}
}

func Negate(i interface{}) interface{} {
	switch v := i.(type) {
	case float32:
		return -v
	case float64:
		return -v
	case int:
		return -v
	case int8:
		return -v
	case int16:
		return -v
	case int32:
		return -v
	case int64:
		return -v
	case uint:
		return -v
	case uint8:
		return -v
	case uint16:
		return -v
	case uint32:
		return -v
	case uint64:
		return -v
	default:
		panic(fmt.Sprintf("invalid operation: - %T", v))
	}
}

func Exponent(a, b interface{}) float64 {
	return math.Pow(ToFloat64(a), ToFloat64(b))
}

func MakeRange(min, max int) []int {
	size := max - min + 1
	if size <= 0 {
		return []int{}
	}
	rng := make([]int, size)
	for i := range rng {
		rng[i] = min + i
	}
	return rng
}

func ToInt(a interface{}) int {
	switch x := a.(type) {
	case float32:
		return int(x)
	case float64:
		return int(x)
	case int:
		return x
	case int8:
		return int(x)
	case int16:
		return int(x)
	case int32:
		return int(x)
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint8:
		return int(x)
	case uint16:
		return int(x)
	case uint32:
		return int(x)
	case uint64:
		return int(x)
	default:
		panic(fmt.Sprintf("invalid operation: int(%T)", x))
	}
}

func ToInt64(a interface{}) int64 {
	switch x := a.(type) {
	case float32:
		return int64(x)
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	default:
		panic(fmt.Sprintf("invalid operation: int64(%T)", x))
	}
}

func ToFloat64(a interface{}) float64 {
	switch x := a.(type) {
	case float32:
		return float64(x)
	case float64:
		return x
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	default:
		panic(fmt.Sprintf("invalid operation: float64(%T)", x))
	}
}

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return r.IsNil()
	default:
		return false
	}
}
