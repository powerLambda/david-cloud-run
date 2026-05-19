package modules

import "net/http"

type Module interface {
	Path() string
	Handler() http.Handler
}

func Register(mux *http.ServeMux, modules ...Module) {
	for _, module := range modules {
		mux.Handle(module.Path(), module.Handler())
	}
}
