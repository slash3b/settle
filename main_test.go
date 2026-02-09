package main

import (
	"testing"
)

func TestPrintVersion(t *testing.T) {
	out := captureOutput(t, func() {
		printVersion()
	})

	assertContains(t, out, "settle")
	assertContains(t, out, Version)
	assertContains(t, out, "binary:")
}

func TestPrintUsage(t *testing.T) {
	out := captureStderr(t, func() {
		printUsage()
	})

	assertContains(t, out, "Usage: settle")
	assertContains(t, out, "Commands:")
	assertContains(t, out, "apply")
	assertContains(t, out, "install")
	assertContains(t, out, "remove")
	assertContains(t, out, "update")
	assertContains(t, out, "list")
	assertContains(t, out, "version")
	assertContains(t, out, "Flags:")
}
