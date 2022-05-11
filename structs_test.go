package structs

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTags(t *testing.T) {
	type MyStruct struct {
		TestString string `foobar:"asdf"`
		TestBool   bool
	}

	s := MyStruct{}
	tags := Tags(s, "foobar")

	assert.Equal(t, map[string]map[string]string{
		"TestString": {"asdf": "true"},
	}, tags)
}

func TestFillMap(t *testing.T) {
	type MyType int
	type MyStruct struct {
		TestString string
		TestBool   bool
		TestType   MyType
	}

	input := MyStruct{"xyz", true, 42}
	output := map[string]string{}

	assert.NoError(t, FillMap(input, output, "tag_name", nil))
	assert.Equal(t, input.TestString, output["TestString"])
	assert.Equal(t, "1", output["TestBool"])
	assert.Equal(t, "42", output["TestType"])
}

func TestFillMapWithDecoder(t *testing.T) {
	type MyStruct struct {
		TestBool bool
	}

	input := MyStruct{true}
	output := map[string]string{}

	assert.NoError(t, FillMap(input, output, "tag_name", func(v reflect.Value, _ reflect.Value, _ map[string]string) (interface{}, error) {
		var s string
		k := v.Kind()
		switch k {
		case reflect.Bool:
			switch v.Bool() {
			case true:
				s = "1"
			case false:
				s = "0"
			}
			return reflect.ValueOf(s).Interface(), nil
		}
		return v.Interface(), nil
	}))
	assert.Equal(t, "1", output["TestBool"])
}

func TestFillStruct(t *testing.T) {
	type MyStruct struct {
		TestString  string
		TestBool    bool
		TestBoolPtr *bool
	}

	input := map[string]string{
		"TestString":  "abc",
		"TestBool":    "true",
		"TestBoolPtr": "1",
	}
	output := MyStruct{}

	assert.NoError(t, fillStruct(input, &output, "tag_name", nil))
	assert.Equal(t, "abc", output.TestString)
	assert.Equal(t, true, output.TestBool)
	assert.Equal(t, true, *output.TestBoolPtr)
}

func TestFormatting(t *testing.T) {
	type MyStruct struct {
		PadLeft        string `mytag:"start=0,end=9,padleft"`
		PadLeftCustom  string `mytag:"start=0,end=9,padleft=0"`
		PadRight       string `mytag:"start=0,end=9,padright"`
		PadRightCustom string `mytag:"start=0,end=9,padright=0"`
		PadLeftInt     int64  `mytag:"start=0,end=9,padleft=0"`
	}

	input := MyStruct{
		PadLeft:        "abc",
		PadLeftCustom:  "abc",
		PadRight:       "abc",
		PadRightCustom: "abc",
		PadLeftInt:     10,
	}
	output := map[string]string{}

	assert.NoError(t, FillMap(input, output, "mytag", nil))
	assert.Equal(t, "       abc", output["PadLeft"])
	assert.Equal(t, "0000000abc", output["PadLeftCustom"])
	assert.Equal(t, "abc       ", output["PadRight"])
	assert.Equal(t, "abc0000000", output["PadRightCustom"])
	assert.Equal(t, "0000000010", output["PadLeftInt"])
}
