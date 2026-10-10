package search

import (
	"errors"
	"math"
)

func validateVector(vector []float32) error {
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return errors.New("vector values must be finite")
		}
	}
	return nil
}
