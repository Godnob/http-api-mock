package notify

import "github.com/Godnob/http-api-mock/definition"

//Notifier notifies the needed parties
type Notifier interface {
	//Notify the needed parties
	Notify(m *definition.Mock) bool
}
