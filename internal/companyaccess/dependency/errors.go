package dependency

import "errors"

var (
	ErrInvalidObjectType = errors.New("invalid object_type")
	ErrObjectNotFound    = errors.New("object not found in company")
)
