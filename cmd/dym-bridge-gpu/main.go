// dym-bridge-gpu is an native GoGPU interface preview.
package main

import (
	"flag"
	"github.com/dymmer-code/dym-bridge/pkg/gpuui"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"log"
)

func main() {
	dir := flag.String("dir", local.DefaultLocalDir(), "Initial local directory")
	configPath := flag.String("config", "", "Path to a dym-bridge INI file")
	flag.Parse()
	if err := gpuui.RunWithConfig(*dir, *configPath); err != nil {
		log.Fatal(err)
	}
}
