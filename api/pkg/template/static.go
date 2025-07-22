package template

/*
  This was taken from https://github.com/replicatedhq/kots/blob/806e07452fe244a15572b759d70b8af2cd67bfaf/pkg/template/static_context.go
*/

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math/big"
	"reflect"
	"regexp/syntax"
	"strconv"
	"strings"
	"time"

	units "github.com/docker/go-units"
	"gopkg.in/yaml.v3"
)

const (
	DefaultRandomStringCharset = "[_A-Za-z0-9]"
)

func (e *Engine) now() string {
	return e.nowFormat("")
}

func (e *Engine) nowFormat(format string) string {
	if format == "" {
		format = time.RFC3339
	}
	return time.Now().UTC().Format(format)
}

func (e *Engine) trim(s string, args ...string) string {
	if len(args) == 0 {
		return strings.TrimSpace(s)
	}
	return strings.Trim(s, args[0])
}

func (e *Engine) base64Encode(plain string) string {
	return base64.StdEncoding.EncodeToString([]byte(plain))
}

func (e *Engine) base64Decode(encoded string) string {
	plain, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(plain)
}

// stolen from https://github.com/replicatedhq/replicated/blob/8ce3ed40436e38b8089387d103623dbe09bbf1c0/pkg/commands/random.go#L22
func (e *Engine) randomString(length uint64, providedCharset ...string) string {
	charset := DefaultRandomStringCharset
	if len(providedCharset) >= 1 {
		charset = providedCharset[0]
	}
	regExp, err := syntax.Parse(charset, syntax.Perl)
	if err != nil {
		return ""
	}

	regExp = regExp.Simplify()
	var b bytes.Buffer
	for i := 0; i < int(length); i++ {
		if err := genString(&b, regExp); err != nil {
			return ""
		}
	}

	result := b.String()
	return result
}

func genString(w *bytes.Buffer, rx *syntax.Regexp) error {
	switch rx.Op {
	case syntax.OpCharClass:
		sum := 0
		for i := 0; i < len(rx.Rune); i += 2 {
			sum += 1 + int(rx.Rune[i+1]-rx.Rune[i])
		}

		for i, nth := 0, rune(randint(sum)); i < len(rx.Rune); i += 2 {
			min, max := rx.Rune[i], rx.Rune[i+1]
			delta := max - min
			if nth <= delta {
				w.WriteRune(min + nth)
				return nil
			}
			nth -= 1 + delta
		}
	default:
		return errors.New("invalid charset")
	}

	return nil
}

func randint(max int) int {
	var bigmax big.Int
	bigmax.SetInt64(int64(max))

	res, err := rand.Int(rand.Reader, &bigmax)
	if err != nil {
		panic(err)
	}

	return int(res.Int64())
}

// RandomBytes returns a base64-encoded byte array allowing the full range of byte values.
func (e *Engine) randomBytes(length uint64) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func (e *Engine) add(a, b interface{}) interface{} {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if isFloat(av) || isFloat(bv) {
		return reflectToFloat(av) + reflectToFloat(bv)
	}
	if isInt(av) {
		return av.Int() + reflectToInt(bv)
	}
	if isUint(av) {
		return av.Uint() + reflectToUint(bv)
	}

	return 0
}

func (e *Engine) sub(a, b interface{}) interface{} {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if isFloat(av) || isFloat(bv) {
		return reflectToFloat(av) - reflectToFloat(bv)
	}
	if isInt(av) {
		return av.Int() - reflectToInt(bv)
	}
	if isUint(av) {
		return av.Uint() - reflectToUint(bv)
	}

	return 0
}

func (e *Engine) mult(a, b interface{}) interface{} {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if isFloat(av) || isFloat(bv) {
		return reflectToFloat(av) * reflectToFloat(bv)
	}
	if isInt(av) {
		return av.Int() * reflectToInt(bv)
	}
	if isUint(av) {
		return av.Uint() * reflectToUint(bv)
	}

	return 0
}

func (e *Engine) div(a, b interface{}) interface{} {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if isFloat(av) || isFloat(bv) {
		return reflectToFloat(av) / reflectToFloat(bv)
	}
	if isInt(av) {
		return av.Int() / reflectToInt(bv)
	}
	if isUint(av) {
		return av.Uint() / reflectToUint(bv)
	}

	return 0
}

func (e *Engine) parseBool(str string) bool {
	val, _ := strconv.ParseBool(str)
	return val
}

func (e *Engine) parseFloat(str string) float64 {
	val, _ := strconv.ParseFloat(str, 64)
	return val
}

func (e *Engine) parseInt(str string, args ...int) int64 {
	base := 10
	if len(args) > 0 {
		base = args[0]
	}
	val, _ := strconv.ParseInt(str, base, 64)
	return val
}

func (e *Engine) parseUint(str string, args ...int) uint64 {
	base := 10
	if len(args) > 0 {
		base = args[0]
	}
	val, _ := strconv.ParseUint(str, base, 64)
	return val
}

func reflectToFloat(val reflect.Value) float64 {
	if isFloat(val) {
		return val.Float()
	}
	if isInt(val) {
		return float64(val.Int())
	}
	if isUint(val) {
		return float64(val.Uint())
	}

	return 0
}

func reflectToInt(val reflect.Value) int64 {
	if isFloat(val) {
		return int64(val.Float())
	}
	if isInt(val) || isUint(val) {
		return val.Int()
	}

	return 0
}

func reflectToUint(val reflect.Value) uint64 {
	if isFloat(val) {
		return uint64(val.Float())
	}
	if isInt(val) || isUint(val) {
		return val.Uint()
	}

	return 0
}

func isFloat(val reflect.Value) bool {
	kind := val.Kind()
	return kind == reflect.Float32 || kind == reflect.Float64
}

func isInt(val reflect.Value) bool {
	kind := val.Kind()
	return kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 || kind == reflect.Int32 || kind == reflect.Int64
}

func isUint(val reflect.Value) bool {
	kind := val.Kind()
	return kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 || kind == reflect.Uint32 || kind == reflect.Uint64
}

func (e *Engine) humanSize(size interface{}) string {
	v := reflect.ValueOf(size)
	return units.HumanSize(reflectToFloat(v))
}

func (e *Engine) yamlEscape(plain string) string {
	marshalled, err := yaml.Marshal(plain)
	if err != nil {
		return ""
	}

	// it is possible for this function to produce multiline yaml, so we indent it a bunch for safety
	indented := indent(20, string(marshalled))
	return indented
}

// copied from sprig
func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}
