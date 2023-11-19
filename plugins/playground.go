package plugins

import (
	"encoding/json"
	"net/http"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/SundaeSwap-finance/bramble"
)

func init() {
	bramble.RegisterPlugin(&PlaygroundPlugin{})
}

type PlaygroundPlugin struct {
	*bramble.BasePlugin
	queryPath string
	config    PlaygroundPluginConfig
}

type PlaygroundPluginConfig struct {
	Path *string `json:"path,omitempty"`
}

func (p *PlaygroundPlugin) Configure(cfg *bramble.Config, data json.RawMessage) error {
	if data == nil {
		return nil
	}
	if err := json.Unmarshal(data, &p.config); err != nil {
		return err
	}
	// We *should* be able to rely on the default being set
	p.queryPath = "/query"
	if cfg.GraphqlPath != nil {
		p.queryPath = *cfg.GraphqlPath
	}
	return nil
}

func (p *PlaygroundPlugin) ID() string {
	return "playground"
}

func (p *PlaygroundPlugin) SetupPublicMux(mux *http.ServeMux) {
	path := "/playground"
	if p.config.Path != nil {
		path = *p.config.Path
	}
	mux.HandleFunc(path, playground.Handler("Bramble Playground", p.queryPath))
}
