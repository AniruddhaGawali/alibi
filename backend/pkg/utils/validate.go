package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

const maxBodyBytes = 1 << 20 // 1 MiB

var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report fields by their JSON name so errors match what the client sent.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return f.Name
		}
		return name
	})
	// password: 8-72 bytes (bcrypt ignores anything past 72) with a letter and a digit.
	_ = v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		pw := fl.Field().String()
		if len(pw) < 8 || len(pw) > 72 {
			return false
		}
		var letter, digit bool
		for _, c := range pw {
			switch {
			case unicode.IsLetter(c):
				letter = true
			case unicode.IsDigit(c):
				digit = true
			}
		}
		return letter && digit
	})
	// username: 3-20 chars of letters, digits and underscore.
	_ = v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return usernameRe.MatchString(fl.Field().String())
	})
	return v
}()

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

// Validate checks dst against its `validate:"..."` struct tags and returns a
// field -> message map, or nil when valid.
func Validate(dst any) map[string]string {
	err := validate.Struct(dst)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return map[string]string{"_": "invalid input"}
	}
	out := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		out[fe.Field()] = fieldMessage(fe)
	}
	return out
}

func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "password":
		return "must be 8-72 characters with at least one letter and one digit"
	case "username":
		return "must be 3-20 characters: letters, digits or underscore"
	case "uuid":
		return "must be a valid UUID"
	default:
		return "is invalid"
	}
}

// BindAndValidate decodes the JSON body into dst (rejecting unknown fields,
// trailing data and oversized bodies) and validates it. On any problem it
// writes the error response itself and returns false; the handler must return.
func BindAndValidate(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		LogInfo("invalid request body", "path", r.URL.Path, "err", err)
		ErrorResponse(w, http.StatusBadRequest, "invalid request body", nil)
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body", nil)
		return false
	}
	if errs := Validate(dst); errs != nil {
		LogInfo("validation failed", "path", r.URL.Path, "fields", len(errs))
		ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", errs)
		return false
	}
	return true
}
