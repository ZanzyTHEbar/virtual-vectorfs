package database

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// vectorZeroString builds a zero vector string for current embedding dims (compact, no spaces)
func (dm *DBManager) vectorZeroString() string {
	dims := dm.config.EmbeddingDims
	if dims <= 0 {
		dims = 4
	}
	buf := make([]byte, 0, 2+(dims*2))
	buf = append(buf, '[')
	for i := 0; i < dims; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendFloat(buf, 0, 'f', -1, 32)
	}
	buf = append(buf, ']')
	return string(buf)
}

// vectorToString converts a float32 array to libSQL vector string format (vector32)
func (dm *DBManager) vectorToString(numbers []float32) (string, error) {
	if len(numbers) == 0 {
		return dm.vectorZeroString(), nil
	}
	dims := dm.config.EmbeddingDims
	if dims <= 0 {
		dims = 4
	}
	if len(numbers) != dims {
		return "", fmt.Errorf("vector must have exactly %d dimensions, got %d", dims, len(numbers))
	}
	buf := make([]byte, 0, 2+(dims*8))
	buf = append(buf, '[')
	for i := 0; i < dims; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		n := numbers[i]
		if math.IsNaN(float64(n)) || math.IsInf(float64(n), 0) {
			n = 0
		}
		buf = strconv.AppendFloat(buf, float64(n), 'f', -1, 32)
	}
	buf = append(buf, ']')
	return string(buf), nil
}

// ExtractVector reads F32_BLOB bytes into []float32
func (dm *DBManager) ExtractVector(ctx context.Context, embedding []byte) ([]float32, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	dims := dm.config.EmbeddingDims
	if dims <= 0 {
		dims = 4
	}
	expected := dims * 4
	if len(embedding) != expected {
		return nil, fmt.Errorf("invalid embedding size: expected %d bytes, got %d", expected, len(embedding))
	}
	vec := make([]float32, dims)
	for i := 0; i < dims; i++ {
		bits := binary.LittleEndian.Uint32(embedding[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}

// coerceToFloat32Slice attempts to interpret arbitrary slice-like inputs as a []float32
func coerceToFloat32Slice(value interface{}) ([]float32, bool, error) {
	switch v := value.(type) {
	case []float32:
		out := make([]float32, len(v))
		copy(out, v)
		return out, true, nil
	case []float64:
		out := make([]float32, len(v))
		for i, n := range v {
			out[i] = float32(n)
		}
		return out, true, nil
	case []int:
		out := make([]float32, len(v))
		for i, n := range v {
			out[i] = float32(n)
		}
		return out, true, nil
	case []int64:
		out := make([]float32, len(v))
		for i, n := range v {
			out[i] = float32(n)
		}
		return out, true, nil
	case []interface{}:
		out := make([]float32, len(v))
		for i, elem := range v {
			switch n := elem.(type) {
			case float64:
				out[i] = float32(n)
			case float32:
				out[i] = n
			case int:
				out[i] = float32(n)
			case int64:
				out[i] = float32(n)
			case string:
				f, err := strconv.ParseFloat(n, 64)
				if err != nil {
					return nil, false, fmt.Errorf("invalid numeric string at %d: %v", i, err)
				}
				out[i] = float32(f)
			default:
				return nil, false, fmt.Errorf("unsupported vector element type at %d: %T", i, elem)
			}
		}
		return out, true, nil
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		n := rv.Len()
		out := make([]float32, n)
		for i := 0; i < n; i++ {
			el := rv.Index(i).Interface()
			switch x := el.(type) {
			case float64:
				out[i] = float32(x)
			case float32:
				out[i] = x
			case int:
				out[i] = float32(x)
			case int64:
				out[i] = float32(x)
			case string:
				f, err := strconv.ParseFloat(x, 64)
				if err != nil {
					return nil, false, fmt.Errorf("invalid numeric string at %d: %v", i, err)
				}
				out[i] = float32(f)
			default:
				return nil, false, fmt.Errorf("unsupported element type at %d: %T", i, el)
			}
		}
		return out, true, nil
	}

	return nil, false, nil
}
