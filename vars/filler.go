package vars

import "github.com/Godnob/http-api-mock/definition"

type Filler interface {
	Fill(m *definition.Mock, input string, multipleMatch bool) string
}
