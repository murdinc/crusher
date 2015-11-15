package servers_test

import (
	"testing"

	"github.com/murdinc/crusher/servers"
	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	server := servers.New("testserver", "127.0.0.1", "wcrusher", "hello_world", false)
	assert.Equal(t, "127.0.0.1", server.Host)
	assert.Equal(t, "testserver", server.Name)
	assert.Equal(t, "hello_world", server.Spec)
	assert.Equal(t, "wcrusher", server.Username)

	assert.False(t, server.PassAuth)
}
