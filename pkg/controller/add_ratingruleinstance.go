package controller

import (
	"github.com/rating-operator-engine/pkg/controller/ratingruleinstance"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, ratingruleinstance.Add)
}
