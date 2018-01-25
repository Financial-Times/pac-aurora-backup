package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractEnvironmentLevel(t *testing.T) {
	env1 := "pac-staging-eu"
	envLevel1, err := extractEnvironmentLevel(env1)
	assert.NoError(t, err)
	assert.Equal(t, "staging", envLevel1)

	env2 := "pac-prod-us"
	envLevel2, err := extractEnvironmentLevel(env2)
	assert.NoError(t, err)
	assert.Equal(t, "prod", envLevel2)
}

func TestExtractEnvironmentLevelError(t *testing.T) {
	_, err := extractEnvironmentLevel("pacstagingeu")
	assert.Error(t, err)

	_, err = extractEnvironmentLevel("pac-us")
	assert.Error(t, err)

	_, err = extractEnvironmentLevel("pac--us")
	assert.Error(t, err)
}
