package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintVersion(t *testing.T) {
	var w strings.Builder

	printVersion(&w)

	out := w.String()

	assert.Contains(t, out, "version")
	assert.Contains(t, out, "settle")

	assert.Contains(t, out, "binary")
}

func TestPrintUsage(t *testing.T) {
	var w strings.Builder

	printUsage(&w)()

	out := w.String()

	assert.Contains(t, out, "Usage: settle")
	assert.Contains(t, out, "Commands:")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "Flags:")
}
