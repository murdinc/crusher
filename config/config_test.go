package config_test

import (
	"testing"

	"github.com/murdinc/crusher/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigRead(t *testing.T) {
	getCfg := func() {
		config.ReadConfig()
	}

	assert.NotPanics(t, getCfg)
}
