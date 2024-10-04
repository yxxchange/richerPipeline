package pkg

import (
	"errors"
)

var (
	ErrTypeAssertFailed = errors.New("type assert failed")
	ErrTypeNotDAG       = errors.New("not a DAG")
	ErrOutOfRange       = errors.New("out of range")
	ErrDataModelConvert = errors.New("data model convert failed")
	ErrDbOperation      = errors.New("db operation failed")
)

type ErrCode int

const (
	ErrCodeGeneralError ErrCode = iota + 1001
	ErrCodeDataStructureError
	ErrCodeDataModelConvertError
	ErrorDbOperationError
)

func ErrorCode(err error) ErrCode {
	switch {
	case errors.Is(err, ErrTypeNotDAG):
		return ErrCodeDataStructureError
	case errors.Is(err, ErrDataModelConvert):
		return ErrCodeDataModelConvertError
	default:
		return ErrCodeGeneralError
	}
}
