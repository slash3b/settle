package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintVersion(t *testing.T) {
	out := captureOutput(t, func() {
		printVersion()
	})

	assert.Contains(t, out, "settle")
	assert.Contains(t, out, Version)
	assert.Contains(t, out, "binary:")
}

func TestPrintUsage(t *testing.T) {
	out := captureStderr(t, func() {
		printUsage()
	})

	assert.Contains(t, out, "Usage: settle")
	assert.Contains(t, out, "Commands:")
	assert.Contains(t, out, "apply")
	assert.Contains(t, out, "install")
	assert.Contains(t, out, "remove")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "Flags:")
}
