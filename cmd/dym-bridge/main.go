package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/lock"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"github.com/dymmer-code/dym-bridge/pkg/ui"
	"github.com/dymmer-code/dym-bridge/pkg/version"
)

func main() {
	configFlag := flag.String("config", "", "Ruta al fichero de configuración dym-bridge.ini")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dym-bridge")
	sampleFlag := flag.Bool("sample-config", false, "Imprime una plantilla de ejemplo dym-bridge.ini")

	encryptFlag := flag.Bool("encrypt", false, "Cifra de forma segura una contraseña o clave SSH para dym-bridge.ini")

	flag.Parse()

	if *versionFlag {
		fmt.Println(version.FullInfo())
		os.Exit(0)
	}

	if *sampleFlag {
		fmt.Print(config.SampleConfigTemplate())
		os.Exit(0)
	}

	if *encryptFlag {
		runCLIEncrypt()
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

func runCLIEncrypt() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Cifrado de credenciales de Dym Bridge (AES-256-GCM) ===")
	fmt.Print("Introduzca el texto o ruta de archivo a cifrar (contraseña o clave SSH): ")
	secretText, _ := reader.ReadString('\n')
	secretText = strings.TrimRight(secretText, "\r\n")
	if secretText == "" {
		fmt.Fprintln(os.Stderr, "Error: el texto no puede estar vacío.")
		os.Exit(1)
	}

	// If the user entered a path to an existing private key file, read the file directly
	if fi, err := os.Stat(secretText); err == nil && !fi.IsDir() {
		bytes, err := os.ReadFile(secretText)
		if err == nil {
			fmt.Printf("Detectado archivo en disco (%s). Se cifrará el contenido del archivo.\n", secretText)
			secretText = string(bytes)
		}
	}

	fmt.Print("Introduzca la clave maestra: ")
	masterPwd, _ := reader.ReadString('\n')
	masterPwd = strings.TrimRight(masterPwd, "\r\n")
	if masterPwd == "" {
		fmt.Fprintln(os.Stderr, "Error: la clave maestra no puede estar vacía.")
		os.Exit(1)
	}

	fmt.Print("Confirme la clave maestra: ")
	confirmPwd, _ := reader.ReadString('\n')
	confirmPwd = strings.TrimRight(confirmPwd, "\r\n")
	if masterPwd != confirmPwd {
		fmt.Fprintln(os.Stderr, "Error: las claves maestras no coinciden.")
		os.Exit(1)
	}

	enc, err := secret.Encrypt(secretText, masterPwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cifrar: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✔ Valor cifrado con éxito (AES-256-GCM).")
	fmt.Println("Copie y pegue la siguiente línea en su archivo dym-bridge.ini:")
	fmt.Printf("\n%s\n\n", enc)
}
