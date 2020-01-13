// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Timeout           time.Duration  `config:"timeout"`
	Path              string         `config:"path"`
	IncludeHeaders    *IncludeConfig `config:"include_headers"`
	LogBackendTraffic bool           `config:"log_backend_traffic"`
}

type IncludeConfig struct {
	ReqHeaders  []string `config:"req"`
	RespHeaders []string `config:"resp"`
	ObjHeaders  []string `config:"obj"`
}

var DefaultConfig = Config{
	IncludeHeaders:    &IncludeConfig{},
	LogBackendTraffic: true,
}
