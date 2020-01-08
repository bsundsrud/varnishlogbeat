// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Timeout time.Duration `config:"timeout"`
	Path    string        `config:"path"`
}

var DefaultConfig = Config{}
