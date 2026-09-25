package workflowdept

import "errors"

var (
	ErrVersionConflict    = errors.New("MAPPING_VERSION_CONFLICT")
	ErrNotFound           = errors.New("mapping not found")
	ErrStepTokenMismatch  = errors.New("STEP_TOKEN_MISMATCH")
	ErrDeptNotInCompany   = errors.New("DEPARTMENT_NOT_IN_COMPANY")
	ErrDeptInactive       = errors.New("DEPARTMENT_INACTIVE")
)

// VersionAction is the result of comparing an expected version with the open row.
type VersionAction string

const (
	ActionCreate     VersionAction = "create"
	ActionIdempotent VersionAction = "idempotent"
	ActionReplace    VersionAction = "replace"
	ActionClose      VersionAction = "close"
)

// PlanWrite decides how a PUT should land. expectedVersion 0 means no open row yet.
func PlanWrite(openVersion int64, hasOpen bool, openDepartmentID, nextDepartmentID string, expectedVersion int64) (VersionAction, error) {
	if !hasOpen {
		if expectedVersion != 0 {
			return "", ErrVersionConflict
		}
		return ActionCreate, nil
	}
	if openVersion != expectedVersion {
		return "", ErrVersionConflict
	}
	if openDepartmentID == nextDepartmentID {
		return ActionIdempotent, nil
	}
	return ActionReplace, nil
}

// PlanClose decides a DELETE. Same version rules as replace, without a new target.
func PlanClose(openVersion int64, hasOpen bool, expectedVersion int64) (VersionAction, error) {
	if !hasOpen {
		return "", ErrNotFound
	}
	if openVersion != expectedVersion {
		return "", ErrVersionConflict
	}
	return ActionClose, nil
}
