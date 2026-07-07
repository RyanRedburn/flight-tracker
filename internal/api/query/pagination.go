package query

import (
	"reflect"

	"github.com/RyanRedburn/flight-tracker/internal/validation"

	"github.com/go-playground/validator/v10"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 500
)

func init() {
	_ = validation.RegisterValidation("list_limit", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.Int {
			return false
		}

		n := fl.Field().Int()

		return n >= 1 && n <= MaxListLimit
	})
}
