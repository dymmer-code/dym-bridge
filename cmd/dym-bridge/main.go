package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/lock"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
	"github.com/dymmer-code/dym-bridge/pkg/ui"
	"github.com/dymmer-code/dym-bridge/pkg/version"
	"golang.org/x/term"
)

func main() {
	configFlag := flag.String("config", "", "Ruta al fichero de configuración dym-bridge.ini")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dym-bridge")
	sampleFlag := flag.Bool("sample-config", false, "Imprime una plantilla de ejemplo dym-bridge.ini")

	encryptFlag := flag.Bool("encrypt", false, "Cifra de forma segura una contraseña o clave SSH para dym-bridge.ini")

	serverFlag := flag.String("server", ui.DefaultServer, "Dymmer server to sign in with (command: login)")

	flag.Parse()

	// The operating system starts dym-bridge with the address the browser opened (dym-bridge://callback?...): it is
	// handed to the process that is waiting for the sign-in, and this one ends.
	for _, arg := range flag.Args() {
		if strings.HasPrefix(arg, "dym-bridge://") {
			os.Exit(deliverCallback(arg))
		}
	}

	if flag.Arg(0) == "login" {
		os.Exit(runCLILogin(*serverFlag))
	}

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

	// Without a .ini, the resources Dymmer gave this device (if it was signed in) are used.
	var profiles *provision.Profiles
	if loadedCfg == nil {
		if path, perr := provision.DefaultPath(); perr == nil {
			if p, lerr := provision.Load(path); lerr == nil {
				profiles = p
			} else if !errors.Is(lerr, provision.ErrNoProfiles) {
				fmt.Fprintf(os.Stderr, "Aviso: %v\n", lerr)
			}
		}
	}

	app := ui.NewApp(loadedCfg, profiles)
	app.Run()
}

func deliverCallback(uri string) int {
	path, err := callback.DefaultPath()
	if err == nil {
		err = callback.Send(path, uri)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runCLILogin signs in from the terminal: the same flow as the button of the window.
func runCLILogin(server string) int {
	fmt.Print("Master password (protects the key of this computer): ")
	pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil || len(pwd) == 0 {
		fmt.Fprintln(os.Stderr, "The master password cannot be empty.")
		return 1
	}
	path, err := provision.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cbPath, err := callback.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rcv, err := callback.Listen(cbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = rcv.Close() }()

	fmt.Println("Opening your browser to approve this computer...")
	profiles, err := provision.Login(context.Background(), provision.LoginOptions{
		Server:         server,
		ProfilesPath:   path,
		MasterPassword: string(pwd),
		Open:           provision.OpenBrowser,
		Receiver:       rcv,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Sign-in failed:", err)
		return 1
	}
	fmt.Printf("Done. %d resource(s):\n", len(profiles.Resources))
	for _, r := range profiles.Resources {
		fmt.Printf("  - %s (%s:%d)\n", r.Label(), r.Address, r.Port)
	}
	return 0
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
