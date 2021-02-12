package controller

import (
	"github.com/rating-operator-engine/pkg/controller/ratingrulemodel"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, ratingrulemodel.Add)
}
