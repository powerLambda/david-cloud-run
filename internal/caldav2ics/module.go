package caldav2ics

import (
	"net/http"

	modules "github.com/powerLambda/david-cloud-run/internal"
	"github.com/powerLambda/david-cloud-run/internal/config"
)

type Module struct {
	path    string
	handler http.Handler
}

func NewModule(cfg config.Config, client Client) modules.Module {
	return &Module{
		path:    cfg.EndpointPath,
		handler: NewHandler(cfg, client),
	}
}

func (m *Module) Path() string {
	return m.path
}

func (m *Module) Handler() http.Handler {
	return m.handler
}
