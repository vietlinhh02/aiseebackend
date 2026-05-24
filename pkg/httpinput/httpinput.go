package httpinput

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type ErrorKind string

const (
	ErrorKindDecode   ErrorKind = "decode"
	ErrorKindValidate ErrorKind = "validate"
)

type Error struct {
	Kind  ErrorKind
	Field string
	Tag   string
	Err   error
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

var validate = validator.New(validator.WithRequiredStructEnabled())

func DecodeAndValidate(r *http.Request, dst any) *Error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &Error{Kind: ErrorKindDecode, Err: err}
	}
	if err := validate.Struct(dst); err != nil {
		if validationErrs, ok := err.(validator.ValidationErrors); ok && len(validationErrs) > 0 {
			first := validationErrs[0]
			return &Error{
				Kind:  ErrorKindValidate,
				Field: first.Field(),
				Tag:   first.Tag(),
				Err:   err,
			}
		}
		return &Error{Kind: ErrorKindValidate, Err: err}
	}
	return nil
}
