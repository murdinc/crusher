package config_test

import (
	"testing"

	"github.com/murdinc/crusher/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigRead(t *testing.T) {

	cfg, err := config.ReadConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg.Servers)

}
