package app

import "github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"

type GetObjectDependenciesRequest struct {
	Subject          AdminSubject
	ObjectType       string
	ObjectID         string
	SampleLimit      int
	IncludeCounts    bool
	IncludeCountsSet bool
}

// ObjectDependenciesView is the API response alias.
type ObjectDependenciesView = dependency.Result
