package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/gpuui"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/lock"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/version"
	"golang.org/x/term"
)

func main() {
	configFlag := flag.String("config", "", "Path to a dym-bridge INI configuration")
	versionFlag := flag.Bool("version", false, "Show the dym-bridge version")
	sampleFlag := flag.Bool("sample-config", false, "Print a sample dym-bridge.ini")

	serverFlag := flag.String("server", gpuui.DefaultServer, "Dymmer server to sign in with (command: login)")

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

	// Ensure only one instance of dym-bridge runs concurrently
	releaseLock, err := lock.AcquireInstanceLock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  %v\n", err)
		os.Exit(1)
	}
	defer releaseLock()

	if err := gpuui.RunWithConfig(local.DefaultLocalDir(), *configFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		releaseLock()
		os.Exit(1)
	}
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
	fmt.Print("Master password (optional; blank stores the key unencrypted): ")
	pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
