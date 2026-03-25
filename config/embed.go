package config

import _ "embed"

//go:embed swarm.key
var MainNetSwarmKey []byte

//go:embed testswarm.key
var TestNetSwarmKey []byte
