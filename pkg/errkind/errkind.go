package errkind

import (
	"errors"
	"fmt"
)

type ExitCode int

const (
	OK ExitCode = iota
	ErrGeneric
	ErrSyntax
	ErrStructure
	ErrUnknownPrimitive
	ErrInvalidSignature
	ErrSandbox
	ErrRuntime
	ErrWorkflowNotFound
	ErrCallableNotFound
)

const (
	TypeGeneric          = "generic"
	TypeSyntax           = "syntax"
	TypeStructure        = "structure"
	TypeUnknownPrimitive = "unknown_primitive"
	TypeInvalidSignature = "invalid_signature"
	TypeSandbox          = "sandbox"
	TypeRuntime          = "runtime"
	TypeWorkflowNotFound = "workflow_not_found"
	TypeCallableNotFound = "callable_not_found"
)

type Error struct {
	Code      ExitCode
	Type      string
	Message   string
	Primitive string
	Position  int
	Callable  string
	Err       error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Type
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(code ExitCode, typ, message string) *Error {
	return &Error{
		Code:    code,
		Type:    typ,
		Message: message,
	}
}

func Wrap(code ExitCode, typ, message string, err error) *Error {
	return &Error{
		Code:    code,
		Type:    typ,
		Message: message,
		Err:     err,
	}
}

func WithContext(err error, primitive string, position int, callable string) *Error {
	if err == nil {
		return nil
	}
	var target *Error
	if errors.As(err, &target) {
		cloned := *target
		if primitive != "" {
			cloned.Primitive = primitive
		}
		if position > 0 {
			cloned.Position = position
		}
		if callable != "" {
			cloned.Callable = callable
		}
		return &cloned
	}
	return &Error{
		Code:      ErrGeneric,
		Type:      TypeGeneric,
		Message:   err.Error(),
		Primitive: primitive,
		Position:  position,
		Callable:  callable,
		Err:       err,
	}
}

func ExitStatus(err error) int {
	var target *Error
	if errors.As(err, &target) {
		return int(target.Code)
	}
	if err == nil {
		return int(OK)
	}
	return int(ErrGeneric)
}

func As(err error, target **Error) bool {
	return errors.As(err, target)
}

func TypeForCode(code ExitCode) string {
	switch code {
	case ErrSyntax:
		return TypeSyntax
	case ErrStructure:
		return TypeStructure
	case ErrUnknownPrimitive:
		return TypeUnknownPrimitive
	case ErrInvalidSignature:
		return TypeInvalidSignature
	case ErrSandbox:
		return TypeSandbox
	case ErrRuntime:
		return TypeRuntime
	case ErrWorkflowNotFound:
		return TypeWorkflowNotFound
	case ErrCallableNotFound:
		return TypeCallableNotFound
	default:
		return TypeGeneric
	}
}

func Format(err error) string {
	var target *Error
	if !errors.As(err, &target) {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	msg := target.Message
	if msg == "" && target.Err != nil {
		msg = target.Err.Error()
	}
	if msg == "" {
		msg = target.Type
	}

	if target.Primitive == "" && target.Position == 0 && target.Callable == "" {
		return msg
	}

	suffix := ""
	if target.Primitive != "" {
		suffix = fmt.Sprintf("primitive=%s", target.Primitive)
	}
	if target.Position > 0 {
		if suffix != "" {
			suffix += " "
		}
		suffix += fmt.Sprintf("position=%d", target.Position)
	}
	if target.Callable != "" {
		if suffix != "" {
			suffix += " "
		}
		suffix += fmt.Sprintf("callable=%s", target.Callable)
	}
	if suffix == "" {
		return msg
	}
	return fmt.Sprintf("%s (%s)", msg, suffix)
}
