// Package main implements the dprint-plugin-shfmt Wasm entrypoint.
package main

import "github.com/hrko/dprint-plugin-shfmt/dprint"

//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-main-boilerplate -runtime runtime -out main_generated.go

var (
	Version    string
	ReleaseTag string
)

type handler struct{}

var runtime = dprint.NewRuntime(&handler{})
