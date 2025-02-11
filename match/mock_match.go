package match

import (
	"errors"
	"strings"

	"github.com/Godnob/http-api-mock/definition"
	urlmatcher "github.com/azer/url-router"
	"github.com/ryanuber/go-glob"
)

var (
	ErrMethodNotMatch   = errors.New("Method not match")
	ErrPathNotMatch     = errors.New("Path not match")
	ErrQueryStringMatch = errors.New("Query string not match")
	ErrHeadersNotMatch  = errors.New("Headers not match")
	ErrCookiesNotMatch  = errors.New("Cookies not match")
	ErrBodyNotMatch     = errors.New("Body not match")
)

type MockMatch struct {
}

func (mm MockMatch) matchKeyAndValues(reqMap definition.Values, mockMap definition.Values, casesensitive, any_one bool) bool {

	if len(mockMap) > len(reqMap) {

		return false
	}

	if !casesensitive {
		for key, val := range reqMap {
			if lkey := strings.ToLower(key); lkey != key {
				delete(reqMap, key)
				reqMap[lkey] = val
			}
		}
	}

	for key, mval := range mockMap {
		if !casesensitive {
			key = strings.ToLower(key)
		}
		if rval, exists := reqMap[key]; exists {
			if any_one && len(rval) == 1 {
				match := false
				for i := range mval {
					if mval[i] == rval[0] {
						match = true
						break
					}
				}
				if !match {
					return false
				}
			} else {
				if len(mval) > len(rval) {
					return false
				}

				for i, v := range mval {
					if v != rval[i] {
						return false
					}
				}
			}
		} else {
			return false
		}
	}
	return true
}

func (mm MockMatch) matchKeyAndValue(reqMap definition.Cookies, mockMap definition.Cookies) bool {
	if len(mockMap) > len(reqMap) {
		return false
	}
	for key, mval := range mockMap {
		if rval, exists := reqMap[key]; !exists || mval != rval {
			return false
		}
	}
	return true
}

func mockIncludesMethod(mock *definition.Request, method string) bool {
	for _, item := range strings.Split(mock.Method, "|") {
		if item == method {
			return true
		}
	}
	return false
}

func (mm MockMatch) Match(req *definition.Request, mock *definition.Request) (bool, error) {
	routes := urlmatcher.New(mock.Path)

	if !glob.Glob(mock.Path, req.Path) {
		if routes.Match(req.Path) == nil {
			return false, ErrPathNotMatch
		}
	}

	if !mockIncludesMethod(mock, req.Method) {
		return false, ErrMethodNotMatch
	}

	if !mm.matchKeyAndValues(req.QueryStringParameters, mock.QueryStringParameters, true, true) {
		return false, ErrQueryStringMatch
	}

	if !mm.matchKeyAndValue(req.Cookies, mock.Cookies) {
		return false, ErrCookiesNotMatch
	}

	if !mm.matchKeyAndValues(req.Headers, mock.Headers, false, false) {
		return false, ErrHeadersNotMatch
	}

	if len(mock.Body) > 0 && !glob.Glob(mock.Body, req.Body) {
		return false, ErrBodyNotMatch
	}

	return true, nil
}
