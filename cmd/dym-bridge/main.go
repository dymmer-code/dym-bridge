package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/lock"
	"github.com/dymmer-code/dym-bridge/pkg/ui"
	"github.com/dymmer-code/dym-bridge/pkg/version"
)

func main() {
	configFlag := flag.String("config", "", "Ruta al fichero de configuración dym-bridge.ini")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dym-bridge")
	sampleFlag := flag.Bool("sample-config", false, "Imprime una plantilla de ejemplo dym-bridge.ini")

	flag.Parse()

	if *versionFlag {
		fmt.Println(version.FullInfo())
		os.Exit(0)
	}

	if *sampleFlag {
		fmt.Print(config.SampleConfigTemplate())
		os.Exit(0)
	}

	// Ensure only one instance of dym-bridge runs concurrently
	releaseLock, err := lock.AcquireInstanceLock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  %v\n", err)
		os.Exit(1)
	}
	defer releaseLock()

	// Try to locate dym-bridge.ini
	var loadedCfg *config.Config
	iniPath, err := config.FindConfigFile(*configFlag)
	if err == nil {
		cfg, err := config.Load(iniPath)
		if err == nil {
			loadedCfg = cfg
		} else {
			fmt.Fprintf(os.Stderr, "Aviso: no se pudo cargar %s: %v\n", iniPath, err)
		}
	}

	app := ui.NewApp(loadedCfg)
	app.Run()
}
