package pkg

import "errors"

var (
	ErrTypeAssertFailed = errors.New("type assert failed")
	ErrTypeNotDAG       = errors.New("not a DAG")
	ErrOutOfRange       = errors.New("out of range")
)
