package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/KabosuNeko/Futon/internal/tui"
	"github.com/KabosuNeko/Futon/internal/updater"
	tea "github.com/charmbracelet/bubbletea"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("futon %s\n", Version)
			return
		case "update", "--update", "self-update", "--self-update":
			runUpdateCLI(Version)
			return
		case "-h", "--help", "help":
			fmt.Printf("futon %s\n\nUsage:\n  futon              Launch TUI reader\n  futon update       Check and install latest release (curl|bash install.sh)\n  futon --version    Print version\n", Version)
			return
		}
	}

	p := tea.NewProgram(tui.NewAppModel(Version), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runUpdateCLI(currentVersion string) {
	if currentVersion == "dev" {
		fmt.Println("Dev build: installing latest release...")
		fmt.Println("Running install.sh...")
		cmdStr := "curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh -o /tmp/futon_install.sh && bash /tmp/futon_install.sh && rm /tmp/futon_install.sh"
		c := exec.Command("bash", "-c", cmdStr)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Update done. Restart futon to use the new version.")
		return
	}
	available, version, _, err := updater.CheckForUpdate(currentVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		os.Exit(1)
	}
	if !available {
		fmt.Printf("Already up to date (%s)\n", currentVersion)
		return
	}
	fmt.Printf("New version %s available — installing...\n", version)
	cmdStr := "curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh -o /tmp/futon_install.sh && bash /tmp/futon_install.sh && rm /tmp/futon_install.sh"
	c := exec.Command("bash", "-c", cmdStr)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Update done. Restart futon to use the new version.")
}
