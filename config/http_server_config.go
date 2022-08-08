package config

import (
	"fmt"
)

type httpServerConfig struct {
	URL string
	HOST string
	PORT int
}

var host = GetStringEnv("HTTP_SERVER_HOST", "0.0.0.0")
var port = GetIntEnv("HTTP_SERVER_PORT", 5000)

var HttpServerConfig = httpServerConfig{
	URL: fmt.Sprintf("%s:%d", host, port),
	HOST: host,
	PORT: port,
}
