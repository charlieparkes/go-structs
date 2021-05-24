package structs

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func Tags(s interface{}, key string) map[string]map[string]string {
	tags := map[string]map[string]string{}
	for _, field := range Fields(s) {
		tag := field.Tag.Get(key)
		if tag != "" {
			if _, ok := tags[field.Name]; !ok {
				tags[field.Name] = map[string]string{}
			}
			for _, tagPart := range strings.Split(tag, ",") {
				if !strings.ContainsRune(tagPart, '=') {
					tags[field.Name][tagPart] = "true"
				} else {
					kv := strings.SplitN(tagPart, "=", 2)
					tags[field.Name][kv[0]] = kv[1]
				}
			}
		}
	}
	return tags
}

func Name(s interface{}) string {
	return Value(s).Type().Name()
}

func Value(s interface{}) reflect.Value {
	v := reflect.ValueOf(s)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("not a struct")
	}
	return v
}

func Fields(s interface{}) []reflect.StructField {
	t := Value(s).Type()
	fields := []reflect.StructField{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// If field is unexported
		if field.PkgPath != "" {
			continue
		}

		fields = append(fields, field)
	}
	return fields
}

func isPtr(i interface{}) error {
	if v := reflect.ValueOf(i); v.Kind() != reflect.Ptr || !v.Elem().CanAddr() {
		return errors.New("must be a pointer")
	}
	return nil
}

func FillMap(from interface{}, to map[string]string, tagName string, decoder func(reflect.Value, reflect.Value, map[string]string) (interface{}, error)) error {
	if from == nil {
		return nil
	}

	tags := Tags(from, tagName)
	fromVal := Value(from)
	for _, field := range Fields(from) {
		val := fromVal.FieldByName(field.Name)

		var output string
		oVal := reflect.ValueOf(&output)

		t := map[string]string{}
		if val, ok := tags[field.Name]; ok {
			t = val
		}

		if decoder != nil {
			v, err := decoder(val, oVal, t)
			if err != nil {
				return err
			}

			val = reflect.ValueOf(v)
		}

		v, err := format(field.Name, val, oVal, t)
		if err != nil {
			return err
		}
		val = reflect.ValueOf(v)

		if err := decode(field.Name, val, oVal); err != nil {
			return err
		}
		to[field.Name] = output
	}
	return nil
}

func FillStruct(from map[string]string, to interface{}, tagName string, decoder func(reflect.Value, reflect.Value, map[string]string) (interface{}, error)) error {
	if err := isPtr(to); err != nil {
		return err
	}

	tags := Tags(to, tagName)
	toVal := Value(to)
	for _, field := range Fields(to) {
		var inVal reflect.Value
		if v, ok := from[field.Name]; ok {
			inVal = reflect.ValueOf(v)
		} else {
			continue
		}

		f := toVal.FieldByName(field.Name)

		// If the fields have the same type, set and exit early.
		if inVal.Kind() == f.Kind() {
			f.Set(inVal)
			continue
		}

		// Run user's decoder, if provided.
		if decoder != nil {
			t := map[string]string{}
			if val, ok := tags[field.Name]; ok {
				t = val
			}

			v, err := decoder(inVal, f, t)
			if err != nil {
				return err
			}
			f.Set(reflect.ValueOf(v))
		}

		if err := decode(field.Name, inVal, f); err != nil {
			return err
		}
	}
	return nil
}

func decode(name string, from reflect.Value, to reflect.Value) error {
	var err error
	switch k := getKind(to); k {
	case reflect.Bool:
		err = decodeBool(name, from, to)
	case reflect.Interface:
		err = decodeBasic(name, from, to)
	case reflect.String:
		err = decodeString(name, from, to)
	case reflect.Int:
		err = decodeInt(name, from, to)
	case reflect.Float32:
		err = decodeFloat(name, from, to)
	case reflect.Ptr:
		err = decodePtr(name, from, to)
	default:
		return fmt.Errorf("%v: unsupported type: %v", name, k)
	}
	return err
}

func getKind(val reflect.Value) reflect.Kind {
	kind := val.Kind()
	switch {
	case kind >= reflect.Int && kind <= reflect.Int64:
		return reflect.Int
	case kind >= reflect.Uint && kind <= reflect.Uint64:
		return reflect.Uint
	case kind >= reflect.Float32 && kind <= reflect.Float64:
		return reflect.Float32
	default:
		return kind
	}
}

// decodeBool supports bool, int, string
func decodeBool(name string, from reflect.Value, to reflect.Value) error {
	switch k := getKind(from); k {
	case reflect.Bool:
		to.SetBool(from.Bool())
	case reflect.Int:
		to.SetBool(from.Int() != 0)
	case reflect.String:
		b, err := strconv.ParseBool(from.String())
		if err == nil {
			to.SetBool(b)
		} else if from.String() == "" {
			to.SetBool(false)
		} else {
			return fmt.Errorf("cannot parse '%s' as bool: %s", name, err)
		}
	default:
		return getDecodeErr(name, from, to)
	}
	return nil
}

// decodeString supports string, bool, int, float32
func decodeString(name string, from reflect.Value, to reflect.Value) error {
	switch k := getKind(from); k {
	case reflect.String:
		to.SetString(from.String())
	case reflect.Bool:
		if from.Bool() {
			to.SetString("1")
		} else {
			to.SetString("0")
		}
	case reflect.Int:
		to.SetString(strconv.FormatInt(from.Int(), 10))
	case reflect.Float32:
		to.SetString(strconv.FormatFloat(from.Float(), 'f', -1, 64))
	default:
		return getDecodeErr(name, from, to)
	}
	return nil
}

// decodeInt supports int, bool, string
func decodeInt(name string, from reflect.Value, to reflect.Value) error {
	switch k := getKind(from); k {
	case reflect.Int:
		to.SetInt(from.Int())
	case reflect.Bool:
		if from.Bool() {
			to.SetInt(1)
		} else {
			to.SetInt(0)
		}
	case reflect.String:
		s := from.String()
		if s == "" {
			s = "0"
		}
		i, err := strconv.ParseInt(s, 0, to.Type().Bits())
		if err == nil {
			to.SetInt(i)
		} else {
			return fmt.Errorf("cannot parse '%s' as int: %s", name, err)
		}
	default:
		return getDecodeErr(name, from, to)
	}
	return nil
}

// decodeFloat supports float32, int, bool, string
func decodeFloat(name string, from reflect.Value, to reflect.Value) error {
	switch k := getKind(from); k {
	case reflect.Float32:
		to.SetFloat(from.Float())
	case reflect.Int:
		to.SetFloat(float64(from.Int()))
	case reflect.Bool:
		if from.Bool() {
			to.SetFloat(1)
		} else {
			to.SetFloat(0)
		}
	case reflect.String:
		s := from.String()
		if s == "" {
			s = "0"
		}
		f, err := strconv.ParseFloat(s, to.Type().Bits())
		if err == nil {
			to.SetFloat(f)
		} else {
			return fmt.Errorf("cannot parse '%s' as float: %s", name, err)
		}
	default:
		return getDecodeErr(name, from, to)
	}
	return nil
}

func decodePtr(name string, from reflect.Value, to reflect.Value) error {
	// If the input data is nil, then we want to just set the output
	// pointer to be nil as well.
	isNil := from.Interface() == nil
	if !isNil {
		switch v := reflect.Indirect(from); v.Kind() {
		case reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice:
			isNil = v.IsNil()
		}
	}
	if isNil {
		if !to.IsNil() && to.CanSet() {
			nilValue := reflect.New(to.Type()).Elem()
			to.Set(nilValue)
		}

		return nil
	}

	// Create an element of the concrete (non pointer) type and decode
	// into that. Then set the value of the pointer to this type.
	valType := to.Type()
	valElemType := valType.Elem()
	if to.CanSet() {
		realVal := to
		if realVal.IsNil() {
			realVal = reflect.New(valElemType)
		}

		if err := decode(name, from, reflect.Indirect(realVal)); err != nil {
			return err
		}

		to.Set(realVal)
	} else {
		if err := decode(name, from, reflect.Indirect(to)); err != nil {
			return err
		}
	}
	return nil
}

func decodeBasic(name string, from reflect.Value, to reflect.Value) error {
	data := from.Interface()

	if to.IsValid() && to.Elem().IsValid() {
		elem := to.Elem()

		// If we can't address this element, then its not writable. Instead,
		// we make a copy of the value (which is a pointer and therefore
		// writable), decode into that, and replace the whole value.
		copied := false
		if !elem.CanAddr() {
			copied = true

			// Make *T
			copy := reflect.New(elem.Type())

			// *T = elem
			copy.Elem().Set(elem)

			// Set elem so we decode into it
			elem = copy
		}

		// Decode. If we have an error then return. We also return right
		// away if we're not a copy because that means we decoded directly.
		if err := decode(name, from, elem); err != nil || !copied {
			return err
		}

		// If we're a copy, we need to set the final result
		to.Set(elem.Elem())
		return nil
	}

	dataVal := reflect.ValueOf(data)

	// If the input data is a pointer, and the assigned type is the dereference
	// of that exact pointer, then indirect it so that we can assign it.
	// Example: *string to string
	if dataVal.Kind() == reflect.Ptr && dataVal.Type().Elem() == to.Type() {
		dataVal = reflect.Indirect(dataVal)
	}

	if !dataVal.IsValid() {
		dataVal = reflect.Zero(to.Type())
	}

	dataValType := dataVal.Type()
	if !dataValType.AssignableTo(to.Type()) {
		return fmt.Errorf("'%s' expected type '%s', got '%s'", name, to.Type(), dataValType)
	}

	to.Set(dataVal)
	return nil
}

func getDecodeErr(name string, from reflect.Value, to reflect.Value) error {
	return fmt.Errorf("'%s' expected type '%s', got unconvertible type '%s', value: '%v'", name, to.Type(), from.Type(), from.Interface())
}

func format(name string, from reflect.Value, to reflect.Value, tags map[string]string) (interface{}, error) {
	fieldLen := func(t map[string]string) int {
		fail := func() {
			panic(fmt.Sprintf("cannot format field %v without 'start' and 'end' OR 'len' tag", name))
		}

		if v, ok := t["len"]; ok {
			len, err := strconv.ParseInt(v, 10, 0)
			if err != nil {
				panic(err)
			}
			return int(len)
		}

		var err error
		var start, end int64
		if v, ok := t["start"]; ok {
			start, err = strconv.ParseInt(v, 10, 0)
			if err != nil {
				panic(err)
			}
		} else {
			fail()
		}
		if v, ok := t["end"]; ok {
			end, err = strconv.ParseInt(v, 10, 0)
			if err != nil {
				panic(err)
			}
		} else {
			fail()
		}
		return int(end + 1 - start)
	}

	var triggered bool = false
	var s string

	// If we couldn't decode from as a string, skip formatting.
	if err := decodeString(name, from, reflect.ValueOf(&s).Elem()); err != nil {
		return from.Interface(), nil
	}

	padWith := func(s string) string {
		if s == "true" || len(s) != 1 {
			return " "
		}
		return s
	}
	if val, ok := tags["padleft"]; ok {
		triggered = true
		val = padWith(val)
		for len(s) < fieldLen(tags) {
			s = fmt.Sprint(val, s)
		}
	} else if val, ok := tags["padright"]; ok {
		triggered = true
		val = padWith(val)
		for len(s) < fieldLen(tags) {
			s = fmt.Sprint(s, val)
		}
	}

	if triggered {
		return reflect.ValueOf(s).Interface(), nil
	}
	return from.Interface(), nil
}
