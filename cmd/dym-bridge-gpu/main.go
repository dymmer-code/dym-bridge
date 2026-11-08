// dym-bridge-gpu is an isolated, read-only interface experiment.
package main

import (
	"flag"
	"github.com/dymmer-code/dym-bridge/pkg/gpuui"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"log"
)

func main() {
	dir := flag.String("dir", local.DefaultLocalDir(), "Initial local directory (read-only)")
	flag.Parse()
	if err := gpuui.Run(*dir); err != nil {
		log.Fatal(err)
	}
}
