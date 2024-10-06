package common

import (
	"errors"
	"fmt"
)

var (
	ErrTypeAssertFailed = errors.New("type assert failed")
	ErrDataNotDAG       = errors.New("not a DAG")
	ErrOutOfRange       = errors.New("out of range")
	ErrDataModelConvert = errors.New("data model convert failed")
	ErrDbOperation      = errors.New("db operation failed")
	ErrPipelineVersion  = errors.New("unsupported pipeline version")
	ErrPipelineType     = errors.New("unsupported pipeline type")
)

func WrapError(errGroup error, errDetail error) error {
	if errGroup == nil {
		return errDetail
	}
	return fmt.Errorf("%w: %v", errGroup, errDetail)
}

type ErrCode int

const (
	ErrCodeGeneralError ErrCode = iota + 1001
	ErrCodeDataStructureError
	ErrCodeDataModelConvertError
	ErrorCodeDbOperationError
	ErrorCodeNotSupported
)

func ErrorCode(err error) ErrCode {
	switch {
	case errors.Is(err, ErrDataNotDAG):
		return ErrCodeDataStructureError
	case errors.Is(err, ErrDataModelConvert):
		return ErrCodeDataModelConvertError
	case errors.Is(err, ErrDbOperation):
		return ErrorCodeDbOperationError
	case errors.Is(err, ErrPipelineVersion), errors.Is(err, ErrPipelineType):
		return ErrorCodeNotSupported
	default:
		return ErrCodeGeneralError
	}
}
