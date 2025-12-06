// MyApp CLI - Command-line interface
// A simple Hello World CLI application demonstrating multi-binary releases
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/example/myapp/pkg/buildinfo"
)

func main() {
	// Define flags
	versionFlag := flag.Bool("version", false, "Print version information")
	helpFlag := flag.Bool("help", false, "Show help message")
	nameFlag := flag.String("name", "", "Name to greet")
	interactiveFlag := flag.Bool("interactive", false, "Run in interactive mode")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("myapp-cli version %s\n", buildinfo.Version)
		fmt.Printf("  Commit: %s\n", buildinfo.Commit)
		fmt.Printf("  Built:  %s\n", buildinfo.BuildTime)
		fmt.Printf("  Go:     %s\n", runtime.Version())
		fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	// Interactive mode
	if *interactiveFlag {
		runInteractive()
		return
	}

	// Handle commands
	if flag.NArg() == 0 {
		// Default: say hello
		name := *nameFlag
		if name == "" {
			name = "World"
		}
		sayHello(name)
		return
	}

	command := flag.Arg(0)
	args := flag.Args()[1:]

	switch command {
	case "hello":
		name := "World"
		if len(args) > 0 {
			name = strings.Join(args, " ")
		} else if *nameFlag != "" {
			name = *nameFlag
		}
		sayHello(name)
	case "info":
		showInfo()
	case "greet":
		greetCommand(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func sayHello(name string) {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         MyApp CLI - Hello World     │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("  👋 Hello, %s!\n", name)
	fmt.Println()
	fmt.Println("  This is a CLI application that demonstrates")
	fmt.Println("  multi-binary packaging with Releaser.")
	fmt.Println()
	fmt.Println("  Try these commands:")
	fmt.Println("    myapp-cli hello <name>     - Greet someone")
	fmt.Println("    myapp-cli info             - Show system info")
	fmt.Println("    myapp-cli --interactive    - Interactive mode")
	fmt.Println("    myapp-cli --version        - Show version")
	fmt.Println()
	fmt.Println("  For the GUI version, run: myapp-gui")
	fmt.Println()
}

func showInfo() {
	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║           System Information          ║")
	fmt.Println("╠═══════════════════════════════════════╣")
	fmt.Printf("║  OS:        %-25s ║\n", runtime.GOOS)
	fmt.Printf("║  Arch:      %-25s ║\n", runtime.GOARCH)
	fmt.Printf("║  CPUs:      %-25d ║\n", runtime.NumCPU())
	fmt.Printf("║  Go:        %-25s ║\n", runtime.Version())
	fmt.Printf("║  Version:   %-25s ║\n", buildinfo.Version)
	fmt.Println("╚═══════════════════════════════════════╝")
}

func greetCommand(args []string) {
	greetings := []string{
		"Hello", "Hi", "Hey", "Greetings", "Howdy",
		"Bonjour", "Hola", "Ciao", "Namaste", "Salaam",
	}

	name := "World"
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	fmt.Println()
	fmt.Println("🌍 Greetings from around the world:")
	fmt.Println()
	for _, greeting := range greetings {
		fmt.Printf("   %s, %s!\n", greeting, name)
	}
	fmt.Println()
}

func runInteractive() {
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║     MyApp CLI - Interactive Mode              ║")
	fmt.Println("║     Type 'help' for commands, 'exit' to quit  ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("myapp> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "exit", "quit", "q":
			fmt.Println("Goodbye! 👋")
			return
		case "help", "h", "?":
			fmt.Println("Commands:")
			fmt.Println("  hello [name]  - Say hello")
			fmt.Println("  greet [name]  - Greet in multiple languages")
			fmt.Println("  info          - Show system info")
			fmt.Println("  clear         - Clear screen")
			fmt.Println("  exit          - Exit interactive mode")
		case "hello":
			name := "World"
			if len(args) > 0 {
				name = strings.Join(args, " ")
			}
			fmt.Printf("Hello, %s! 👋\n", name)
		case "greet":
			greetCommand(args)
		case "info":
			showInfo()
		case "clear":
			fmt.Print("\033[H\033[2J")
		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func printHelp() {
	fmt.Println(`MyApp CLI - Command-line Hello World Application

Usage:
  myapp-cli [command] [options]

Commands:
  hello [name]   Say hello to someone (default: World)
  greet [name]   Greet someone in multiple languages
  info           Show system information

Options:
  --name <name>     Name to greet
  --interactive     Run in interactive mode
  --version         Print version information
  --help            Show this help message

Examples:
  myapp-cli                      # Say "Hello, World!"
  myapp-cli hello John           # Say "Hello, John!"
  myapp-cli --name Alice         # Say "Hello, Alice!"
  myapp-cli greet Everyone       # Greet in multiple languages
  myapp-cli info                 # Show system info
  myapp-cli --interactive        # Interactive mode

For the graphical version, use: myapp-gui`)
}
