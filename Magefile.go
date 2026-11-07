//go:build mage
// +build mage

package main

import build "github.com/grafana/grafana-plugin-sdk-go/build"

var Default = BuildAll

func BuildAll() {
	build.BuildAll()
}
