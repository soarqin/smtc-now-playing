//go:build !smtc_test

package main

// Version is the current application version.
// Bump this along with a CHANGELOG.md entry. build.bat also embeds it via
// -ldflags "-X main.Version=..." so releases can override at build time.
const Version = "1.2.2"
