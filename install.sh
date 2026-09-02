#!/usr/bin/env bash
set -euo pipefail

REPO="KabosuNeko/Futon"

# ---- Uninstall ----
if [ "${1:-}" = "uninstall" ] || [ "${1:-}" = "--uninstall" ]; then
  echo "Removing futon from /usr/local/bin/ ..."
  sudo rm -f /usr/local/bin/futon
  if [ "$(uname -s)" = "Linux" ]; then
    echo "Removing desktop entry and icon ..."
    sudo rm -f /usr/share/applications/futon.desktop
    sudo rm -f /usr/share/pixmaps/futon.png
    sudo rm -f /usr/share/icons/hicolor/512x512/apps/futon.png
    if command -v update-desktop-database >/dev/null 2>&1; then
      sudo update-desktop-database /usr/share/applications 2>/dev/null || true
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
      sudo gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
    fi
  fi
  echo "Futon uninstalled successfully."
  exit 0
fi

# ---- OS detection ----
case "$(uname -s)" in
  Linux)  OS="linux"  ;;
  Darwin) OS="macOS"  ;;
  *)      echo "Unsupported OS: $(uname -s)"; exit 1 ;;
esac

# ---- Arch detection ----
case "$(uname -m)" in
  x86_64)              ARCH="amd64" ;;
  aarch64|arm64)       ARCH="arm64" ;;
  *)                   echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac

# ---- Fetch latest version ----
echo "Fetching latest release info for $REPO ..."
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed 's/.*"tag_name": "\(.*\)".*/\1/' \
  | sed 's/^v//')

if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version. Aborting."
  exit 1
fi

echo "Latest version: $VERSION"

# ---- Download ----
FILENAME="futon_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/v${VERSION}/$FILENAME"

echo "Downloading $URL ..."
curl -L -o "$FILENAME" "$URL"

# ---- Verify checksum ----
CHECKSUM_URL="https://github.com/$REPO/releases/download/v${VERSION}/checksums.txt"
echo "Downloading checksums from $CHECKSUM_URL ..."
if ! curl -sfL -o checksums.txt "$CHECKSUM_URL"; then
  echo "Failed to download checksums.txt. Cannot verify archive integrity. Aborting."
  rm -f "$FILENAME"
  exit 1
fi

expected=$(awk -v f="$FILENAME" '$2 == f {print $1}' checksums.txt | head -1)
if [ -z "$expected" ]; then
  echo "No checksum found for $FILENAME in checksums.txt. Cannot verify archive integrity. Aborting."
  rm -f "$FILENAME" checksums.txt
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$FILENAME" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$FILENAME" | awk '{print $1}')
fi

if [ "$actual" != "$expected" ]; then
  echo "Checksum mismatch! Aborting."
  echo "Expected: $expected"
  echo "Actual:   $actual"
  rm -f "$FILENAME" checksums.txt
  exit 1
fi

echo "Checksum verified."

# ---- Extract ----
echo "Extracting $FILENAME ..."
tar -xzf "$FILENAME"

# ---- Install ----
echo "Installing futon to /usr/local/bin/ ..."
chmod +x futon
# /usr/local/bin may not exist on macOS (Apple Silicon), create it first
sudo mkdir -p /usr/local/bin
sudo mv futon /usr/local/bin/

# ---- Desktop entry & icon (Linux only) ----
if [ "$OS" = "linux" ]; then
  echo "Installing desktop entry ..."
  DESKTOP_SRC=""
  if [ -f "assets/futon.desktop" ]; then DESKTOP_SRC="assets/futon.desktop";
  elif [ -f "futon.desktop" ]; then DESKTOP_SRC="futon.desktop"; fi
  if [ -n "$DESKTOP_SRC" ] && [ -f "$DESKTOP_SRC" ]; then
    sudo mkdir -p /usr/share/applications
    sudo install -m 644 "$DESKTOP_SRC" /usr/share/applications/futon.desktop
  else
    sudo mkdir -p /usr/share/applications
    cat | sudo tee /usr/share/applications/futon.desktop >/dev/null <<'DESKTOP'
[Desktop Entry]
Name=Futon
Comment=TUI manga reader
GenericName=Manga Reader
Exec=futon
Icon=futon
Terminal=true
Type=Application
Categories=Graphics;Viewer;
Keywords=manga;comic;reader;viewer;
StartupWMClass=futon
StartupNotify=false
DESKTOP
  fi
  ICON_SRC=""
  if [ -f "assets/futon.png" ]; then ICON_SRC="assets/futon.png";
  elif [ -f "futon.png" ]; then ICON_SRC="futon.png";
  fi
  if [ -n "$ICON_SRC" ]; then
    sudo mkdir -p /usr/share/pixmaps /usr/share/icons/hicolor/512x512/apps
    sudo install -m 644 "$ICON_SRC" /usr/share/pixmaps/futon.png
    sudo install -m 644 "$ICON_SRC" /usr/share/icons/hicolor/512x512/apps/futon.png
  else
    echo "Fetching icon ..."
    TMP_ICON=$(mktemp)
    if curl -sfL -o "$TMP_ICON" "https://raw.githubusercontent.com/$REPO/main/assets/futon.png"; then
      sudo mkdir -p /usr/share/pixmaps /usr/share/icons/hicolor/512x512/apps
      sudo install -m 644 "$TMP_ICON" /usr/share/pixmaps/futon.png
      sudo install -m 644 "$TMP_ICON" /usr/share/icons/hicolor/512x512/apps/futon.png
    fi
    rm -f "$TMP_ICON"
  fi
  if command -v update-desktop-database >/dev/null 2>&1; then
    sudo update-desktop-database /usr/share/applications 2>/dev/null || true
  fi
  if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    sudo gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
  fi
fi

# ---- Cleanup ----
rm -f "$FILENAME" checksums.txt
rm -rf assets 2>/dev/null || true
rm -f futon.desktop futon.png 2>/dev/null || true

echo ""
echo "Futon $VERSION installed successfully!"
echo "Run 'futon' to start."
if [ "$OS" = "linux" ]; then
  echo "App launcher: Futon (TUI manga reader)"
fi
