#!/usr/bin/env bash
set -e

# Helper function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

echo "Detecting OS and architecture..."
OS="$(uname -s)"
ARCH="$(uname -m)"

echo "OS: $OS, Arch: $ARCH"

# Detect WSL
is_wsl=false
if grep -qi microsoft /proc/version 2>/dev/null; then
    is_wsl=true
    echo "Detected WSL environment"
fi

install_mac() {
    echo "Running on macOS..."
    if ! command_exists brew; then
        echo "Homebrew not found. Installing..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi

    packages=(pkg-config glfw xquartz)

    for pkg in "${packages[@]}"; do
        if ! brew list "$pkg" &>/dev/null; then
            echo "Installing $pkg..."
            brew install "$pkg" || brew install --cask "$pkg"
        else
            echo "$pkg already installed."
        fi
    done

    export PKG_CONFIG_PATH="/opt/X11/lib/pkgconfig:$PKG_CONFIG_PATH"
}

ensure_gl_pkgconfig_linux() {
    # Some distros split OpenGL headers/pkg-config files across different packages
    # and builds that rely on cgo + OpenGL often fail with: Package 'gl' not found.
    if pkg-config --exists gl; then
        echo "OpenGL pkg-config entry ('gl') already available."
        return
    fi

    echo "OpenGL pkg-config entry ('gl') not found, attempting to install common provider packages..."

    case "$1" in
        apt)
            extras=(mesa-common-dev libgl1-mesa-dev libglu1-mesa-dev libglvnd-dev)
            for pkg in "${extras[@]}"; do
                if ! dpkg -s "$pkg" &>/dev/null; then
                    echo "Installing $pkg..."
                    sudo apt install -y "$pkg" || echo "Warning: could not install $pkg"
                fi
            done
            ;;
        dnf)
            extras=(mesa-libGL-devel mesa-libGLU-devel libglvnd-devel)
            for pkg in "${extras[@]}"; do
                if ! rpm -q "$pkg" &>/dev/null; then
                    echo "Installing $pkg..."
                    sudo dnf install -y "$pkg" || echo "Warning: could not install $pkg"
                fi
            done
            ;;
        pacman)
            extras=(mesa glu libglvnd)
            for pkg in "${extras[@]}"; do
                if ! pacman -Qi "$pkg" &>/dev/null; then
                    echo "Installing $pkg..."
                    sudo pacman -S --noconfirm "$pkg" || echo "Warning: could not install $pkg"
                fi
            done
            ;;
        *)
            echo "Install OpenGL development headers manually so pkg-config can find 'gl'."
            ;;
    esac

    if pkg-config --exists gl; then
        echo "OpenGL pkg-config entry ('gl') detected after installation."
    else
        echo "Warning: pkg-config still cannot find 'gl'. Please install OpenGL development headers for your distribution."
    fi
}

install_ubuntu() {
    echo "Running on Ubuntu/Debian..."
    sudo apt update -qq
    packages=(build-essential pkg-config libx11-dev libxrandr-dev libxi-dev libxcursor-dev libxinerama-dev libgl1-mesa-dev mesa-common-dev libglu1-mesa-dev libglvnd-dev)

    for pkg in "${packages[@]}"; do
        if ! dpkg -s "$pkg" &>/dev/null; then
            echo "Installing $pkg..."
            sudo apt install -y "$pkg"
        else
            echo "$pkg already installed."
        fi
    done

    ensure_gl_pkgconfig_linux apt
}

install_fedora() {
    echo "Running on Fedora..."
    packages=(gcc gcc-c++ make pkg-config libX11-devel libXrandr-devel libXi-devel libXcursor-devel libXinerama-devel mesa-libGL-devel mesa-libGLU-devel libglvnd-devel)

    for pkg in "${packages[@]}"; do
        if ! rpm -q "$pkg" &>/dev/null; then
            echo "Installing $pkg..."
            sudo dnf install -y "$pkg"
        else
            echo "$pkg already installed."
        fi
    done

    ensure_gl_pkgconfig_linux dnf
}

install_arch() {
    echo "Running on Arch Linux..."
    packages=(base-devel pkgconf libx11 libxrandr libxi libxcursor libxinerama mesa glu libglvnd)

    for pkg in "${packages[@]}"; do
        if ! pacman -Qi "$pkg" &>/dev/null; then
            echo "Installing $pkg..."
            sudo pacman -S --noconfirm "$pkg"
        else
            echo "$pkg already installed."
        fi
    done

    ensure_gl_pkgconfig_linux pacman
}

install_other_linux() {
    echo "Running on other Linux distro. Please install manually:"
    echo "- X11 headers (Xlib.h)"
    echo "- OpenGL development libraries"
    echo "- pkg-config"
}

# Main logic
case "$OS" in
    Darwin)
        install_mac
        ;;
    Linux)
        if [ "$is_wsl" = true ]; then
            echo "WSL detected. Installing Ubuntu packages..."
            install_ubuntu
        elif [ -f /etc/debian_version ]; then
            install_ubuntu
        elif [ -f /etc/fedora-release ]; then
            install_fedora
        elif [ -f /etc/arch-release ]; then
            install_arch
        else
            install_other_linux
        fi
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

echo "All dependencies installed. Ready to build your Go project."
