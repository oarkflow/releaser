// MyApp GUI - Graphical User Interface
// A Hello World GUI application using Fyne toolkit
// This demonstrates a cross-platform desktop application
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Check for version flag (CLI mode)
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("MyApp GUI version %s\n", version)
			fmt.Printf("  Commit: %s\n", commit)
			fmt.Printf("  Built:  %s\n", date)
			fmt.Printf("  Go:     %s\n", runtime.Version())
			fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			os.Exit(0)
		}
		if arg == "--help" || arg == "-h" {
			fmt.Println("MyApp GUI - Graphical Hello World Application")
			fmt.Println()
			fmt.Println("Usage: myapp-gui [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --version, -v    Show version information")
			fmt.Println("  --help, -h       Show this help message")
			fmt.Println()
			fmt.Println("For CLI version, use: myapp-cli")
			os.Exit(0)
		}
	}

	// Create and run the GUI application
	runGUI()
}

func runGUI() {
	// Create the Fyne application
	a := app.NewWithID("com.myapp.gui")
	a.SetIcon(theme.FyneLogo())

	// Create the main window
	w := a.NewWindow("MyApp - Hello World")
	w.Resize(fyne.NewSize(500, 400))
	w.CenterOnScreen()

	// Create greeting label
	greetingLabel := widget.NewLabelWithStyle(
		"👋 Hello, World!",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Name entry
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Enter your name...")

	// Greeting button
	greetBtn := widget.NewButton("Say Hello!", func() {
		name := nameEntry.Text
		if name == "" {
			name = "World"
		}
		greetingLabel.SetText(fmt.Sprintf("👋 Hello, %s!", name))
	})
	greetBtn.Importance = widget.HighImportance

	// Create a clock that updates
	clockLabel := widget.NewLabel(time.Now().Format("15:04:05"))
	go func() {
		for range time.Tick(time.Second) {
			clockLabel.SetText(time.Now().Format("15:04:05"))
		}
	}()

	// System info
	infoText := fmt.Sprintf("Platform: %s/%s | Go: %s | Version: %s",
		runtime.GOOS, runtime.GOARCH, runtime.Version(), version)
	infoLabel := widget.NewLabel(infoText)
	infoLabel.Alignment = fyne.TextAlignCenter

	// Title with styling
	title := canvas.NewText("MyApp GUI", theme.ForegroundColor())
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := widget.NewLabel("A Hello World Desktop Application")
	subtitle.Alignment = fyne.TextAlignCenter

	// Create menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("New Greeting", func() {
				nameEntry.SetText("")
				greetingLabel.SetText("👋 Hello, World!")
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() {
				a.Quit()
			}),
		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About", func() {
				showAboutDialog(w)
			}),
		),
	)
	w.SetMainMenu(mainMenu)

	// Language greetings
	greetings := []string{
		"Hello", "Bonjour", "Hola", "Ciao", "Namaste",
		"Konnichiwa", "Annyeong", "Merhaba", "Shalom", "Salaam",
	}

	// Multi-language button
	multiLangBtn := widget.NewButton("🌍 Greet in All Languages", func() {
		name := nameEntry.Text
		if name == "" {
			name = "World"
		}
		showMultiLanguageDialog(w, name, greetings)
	})

	// Layout the content
	header := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
		layout.NewSpacer(),
	)

	middleContent := container.NewVBox(
		container.NewCenter(greetingLabel),
		layout.NewSpacer(),
		container.NewHBox(
			layout.NewSpacer(),
			container.NewGridWithColumns(2,
				nameEntry,
				greetBtn,
			),
			layout.NewSpacer(),
		),
		container.NewCenter(multiLangBtn),
		layout.NewSpacer(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			layout.NewSpacer(),
			clockLabel,
			layout.NewSpacer(),
		),
		container.NewCenter(infoLabel),
	)

	content := container.NewBorder(
		header,
		footer,
		nil,
		nil,
		middleContent,
	)

	w.SetContent(content)

	// Handle window close
	w.SetCloseIntercept(func() {
		dialog.ShowConfirm("Exit", "Are you sure you want to exit?", func(confirm bool) {
			if confirm {
				a.Quit()
			}
		}, w)
	})

	// Show and run
	w.ShowAndRun()
}

func showAboutDialog(w fyne.Window) {
	aboutText := fmt.Sprintf(`MyApp GUI
Version: %s

A Hello World Desktop Application
demonstrating multi-binary packaging
with Releaser.

Built with Fyne (https://fyne.io)

Platform: %s/%s
Go: %s
Commit: %s
Built: %s`,
		version, runtime.GOOS, runtime.GOARCH,
		runtime.Version(), commit, date)

	dialog.ShowInformation("About MyApp", aboutText, w)
}

func showMultiLanguageDialog(w fyne.Window, name string, greetings []string) {
	var items []fyne.CanvasObject
	for _, greeting := range greetings {
		label := widget.NewLabel(fmt.Sprintf("%s, %s!", greeting, name))
		label.Alignment = fyne.TextAlignCenter
		items = append(items, label)
	}

	content := container.NewVBox(items...)
	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(300, 200))

	dialog.ShowCustom("🌍 Greetings from Around the World", "Close", scroll, w)
}
