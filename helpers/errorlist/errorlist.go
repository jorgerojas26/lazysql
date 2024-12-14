package errorlist

import (
	"fmt"
)

type ErrorList []error

func (e ErrorList) Error() string {
	s := ""

	for idx, err := range e {
		s = fmt.Sprintf("%d: %s\n", idx, err)
	}

	return s
}

// Append will append an error into [ErrorList]
func (e ErrorList) Append(err error) ErrorList {
	return append(e, err)
}

// New will create new instance of [ErrorList]
//
// use [nil] as parameter to create empty [ErrorList]
//
// use [Append] to add into [ErrorList]
func New(err error) ErrorList {
	if err == nil {
		return make(ErrorList, 0)
	}

	if errl, ok := err.(ErrorList); ok {
		return errl
	}

	e := make(ErrorList, 0)
	e = append(e, err)
	return e
}
