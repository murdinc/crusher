package specr_test

import (
	"testing"

	"github.com/murdinc/crusher/specr"
	"github.com/stretchr/testify/assert"
)

func TestGetSpecs(t *testing.T) {
	specList, err := specr.GetSpecs()

	assert.NoError(t, err)
	assert.True(t, specList.SpecExists("hello_world"))

}
