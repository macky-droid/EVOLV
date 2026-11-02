#!/bin/sh
# ============================================================================
#  EVOLV installer — the workspace that rewrites itself
#  curl -fsSL https://raw.githubusercontent.com/dxn111/EVOLV/main/install.sh | sh
# ============================================================================
set -eu

REPO="dxn111/EVOLV"
BIN_NAME="evolv"
INSTALL_DIR_LOCAL="$HOME/.local/bin"
VERSION="${EVOLV_VERSION:-latest}"

# ── terminal capability ─────────────────────────────────────────────────────
UI=0
if [ -t 1 ] && [ -n "${TERM:-}" ] && [ "${TERM}" != "dumb" ] && [ -z "${NO_COLOR:-}" ]; then
  UI=1
  ESC=$(printf '\033')
  BOLD="${ESC}[1m"; DIM="${ESC}[2m"; RESET="${ESC}[0m"
  VIOLET="${ESC}[38;5;141m"; CYAN="${ESC}[38;5;80m"; PINK="${ESC}[38;5;213m"
  GREEN="${ESC}[38;5;114m"; YELLOW="${ESC}[38;5;221m"; RED="${ESC}[38;5;203m"
else
  BOLD=""; DIM=""; RESET=""; VIOLET=""; CYAN=""; PINK=""; GREEN=""; YELLOW=""; RED=""
fi

cleanup() {
  if [ "$UI" = "1" ]; then printf '\033[?25h'; fi
}
trap cleanup EXIT
trap 'cleanup; exit 130' INT TERM

say()  { printf "%s\n" "$*"; }
ok()   { printf "  %s✔%s %s\n" "$GREEN" "$RESET" "$*"; }
warn() { printf "  %s!%s %s\n" "$YELLOW" "$RESET" "$*"; }
die()  { printf "  %s✘%s %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

# ── animated banner ─────────────────────────────────────────────────────────
typewrite() {
  text="$1"
  if [ "$UI" != "1" ]; then printf '%s' "$text"; return 0; fi
  i=0
  len=${#text}
  while [ "$i" -lt "$len" ]; do
    i=$((i + 1))
    printf '%s' "$(printf '%s' "$text" | cut -c "$i")"
    sleep 0.012
  done
}

banner_row() {
  printf '%s%s%s\n' "$1" "$2" "$RESET"
  if [ "$UI" = "1" ]; then sleep 0.035; fi
}

banner() {
  if [ "$UI" = "1" ]; then printf '\033[?25l'; fi
  printf "\n"
  banner_row "$VIOLET" '  ███████╗██╗   ██╗ ██████╗ ██╗    ██╗   ██╗'
  banner_row "$VIOLET" '  ██╔════╝██║   ██║██╔═══██╗██║    ██║   ██║'
  banner_row "$CYAN"   '  █████╗  ██║   ██║██║   ██║██║    ██║   ██║'
  banner_row "$CYAN"   '  ██╔══╝  ╚██╗ ██╔╝██║   ██║██║    ██║   ██║'
  banner_row "$PINK"   '  ███████╗ ╚████╔╝ ╚██████╔╝███████╗╚██████╔╝'
  banner_row "$PINK"   '  ╚══════╝  ╚═══╝   ╚═════╝ ╚══════╝ ╚═════╝'
  printf "\n  %s" "$DIM"
  typewrite "the workspace that rewrites itself"
  printf "%s\n" "$RESET"
  printf "  %s\n\n" "${DIM}github.com/${REPO}${RESET}"
  if [ "$UI" = "1" ]; then printf '\033[?25h'; fi
}

# ── progress rendering ──────────────────────────────────────────────────────
BAR_W=22
LABEL_W=11

frame_char() {
  case $(( $1 % 10 )) in
    0) printf '⠋' ;; 1) printf '⠙' ;; 2) printf '⠹' ;; 3) printf '⠸' ;;
    4) printf '⠼' ;; 5) printf '⠴' ;; 6) printf '⠦' ;; 7) printf '⠧' ;;
    8) printf '⠇' ;; 9) printf '⠏' ;;
  esac
}

bar_fill() {
  n="$1"; i=0
  while [ "$i" -lt "$n" ]; do printf '█'; i=$((i + 1)); done
}

bar_void() {
  n="$1"; i=0
  while [ "$i" -lt "$n" ]; do printf '░'; i=$((i + 1)); done
}

draw_bar() {
  label="$1"; pct="$2"; secs="$3"; frame="$4"
  filled=$((pct * BAR_W / 100))
  empty=$((BAR_W - filled))
  printf '\r  %s%s%s %-*s  %s%s%s%s%s  %3d%%  %s%ss%s\033[K' \
    "$CYAN" "$(frame_char "$frame")" "$RESET" \
    "$LABEL_W" "$label" \
    "$CYAN" "$(bar_fill "$filled")" "$DIM" "$(bar_void "$empty")" "$RESET" \
    "$pct" "$DIM" "$secs" "$RESET"
}

draw_spin() {
  label="$1"; secs="$2"; frame="$3"
  printf '\r  %s%s%s %-*s  %s%ss%s\033[K' \
    "$CYAN" "$(frame_char "$frame")" "$RESET" "$LABEL_W" "$label" \
    "$DIM" "$secs" "$RESET"
}

draw_done() {
  label="$1"; detail="$2"; secs="$3"
  if [ -n "$detail" ]; then
    printf '\r  %s✔%s %-*s  %s%s%s  %s(%ss)%s\033[K\n' \
      "$GREEN" "$RESET" "$LABEL_W" "$label" "$DIM" "$detail" "$RESET" \
      "$DIM" "$secs" "$RESET"
  else
    printf '\r  %s✔%s %-*s  %s(%ss)%s\033[K\n' \
      "$GREEN" "$RESET" "$LABEL_W" "$label" "$DIM" "$secs" "$RESET"
  fi
}

draw_fail() {
  label="$1"; detail="$2"
  printf '\r  %s✘%s %-*s  %s%s%s\033[K\n' \
    "$RED" "$RESET" "$LABEL_W" "$label" "$DIM" "$detail" "$RESET"
}

draw_warn() {
  label="$1"; detail="$2"
  printf '\r  %s!%s %-*s  %s%s%s\033[K\n' \
    "$YELLOW" "$RESET" "$LABEL_W" "$label" "$DIM" "$detail" "$RESET"
}

# ── stages ──────────────────────────────────────────────────────────────────
run_stage() {
  # $1 label · $2 command · $3 success detail · $4 "bar" (default) or "spin"
  # $5 "soft" turns an expected failure into a yellow note instead of a red error
  label="$1"; cmd="$2"; detail="${3:-}"; mode="${4:-bar}"; soft="${5:-}"
  if [ "$UI" != "1" ]; then
    printf "  … %s\n" "$label"
    set +e
    eval "$cmd"
    rc=$?
    set -e
    if [ "$rc" -eq 0 ]; then
      printf "  ok %s\n" "$label"
    elif [ "$soft" = "soft" ]; then
      printf "  ! %s — falling back\n" "$label"
    else
      printf "  FAILED %s\n" "$label"
    fi
    return "$rc"
  fi
  log="${TMPDIR:-/tmp}/evolv-stage.$$"
  start=$(date +%s)
  ( eval "$cmd" ) >"$log" 2>&1 &
  pid=$!
  pct=0
  frame=0
  while kill -0 "$pid" 2>/dev/null; do
    secs=$(( $(date +%s) - start ))
    if [ "$mode" = "bar" ]; then
      pct=$((pct + 4))
      if [ "$pct" -gt 94 ]; then pct=94; fi
      draw_bar "$label" "$pct" "$secs" "$frame"
    else
      draw_spin "$label" "$secs" "$frame"
    fi
    frame=$((frame + 1))
    sleep 0.06
  done
  set +e
  wait "$pid"
  rc=$?
  set -e
  secs=$(( $(date +%s) - start ))
  if [ "$rc" -eq 0 ]; then
    draw_done "$label" "$detail" "$secs"
  elif [ "$soft" = "soft" ]; then
    draw_warn "$label" "no prebuilt — building from source"
  else
    draw_fail "$label" "failed after ${secs}s"
    if [ -s "$log" ]; then
      tail -n 12 "$log" | sed 's/^/      /'
    fi
  fi
  rm -f "$log"
  return "$rc"
}

run_capture() {
  # like run_stage, but the command's stdout is captured into $3
  label="$1"; cmd="$2"; out="$3"
  if [ "$UI" != "1" ]; then
    printf "  … %s\n" "$label"
    set +e
    eval "$cmd" >"$out" 2>/dev/null
    rc=$?
    set -e
    if [ "$rc" -eq 0 ]; then printf "  ok %s\n" "$label"; else printf "  FAILED %s\n" "$label"; fi
    return "$rc"
  fi
  start=$(date +%s)
  ( eval "$cmd" ) >"$out" 2>/dev/null &
  pid=$!
  frame=0
  while kill -0 "$pid" 2>/dev/null; do
    secs=$(( $(date +%s) - start ))
    draw_spin "$label" "$secs" "$frame"
    frame=$((frame + 1))
    sleep 0.06
  done
  set +e
  wait "$pid"
  rc=$?
  set -e
  secs=$(( $(date +%s) - start ))
  if [ "$rc" -eq 0 ]; then
    detail=$(head -n 1 "$out" 2>/dev/null | tr -d '\r\n')
    draw_done "$label" "$detail" "$secs"
  else
    draw_fail "$label" "failed after ${secs}s"
  fi
  return "$rc"
}

# ── detect platform ─────────────────────────────────────────────────────────
detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7*|armv6*) ARCH="arm" ;;
    *) die "unsupported architecture: $ARCH" ;;
  esac
  case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) die "unsupported OS: $OS (windows: use 'go install github.com/${REPO}/evolv@latest' or WSL)" ;;
  esac
}

# ── fetch ───────────────────────────────────────────────────────────────────
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    die "need curl or wget to install"
  fi
}

# ── resolve latest release tag ──────────────────────────────────────────────
latest_tag() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed 's#.*/tag/##'
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- --max-redirect=0 "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed -n 's#.*/tag/##p' | head -1
  fi
}

# ── install paths ───────────────────────────────────────────────────────────
choose_install_dir() {
  # prefer ~/.local/bin (no sudo), fall back to /usr/local/bin, then ./bin
  if mkdir -p "$INSTALL_DIR_LOCAL" 2>/dev/null && [ -w "$INSTALL_DIR_LOCAL" ]; then
    INSTALL_DIR="$INSTALL_DIR_LOCAL"
  elif [ -w /usr/local/bin ] || [ "$(id -u)" = "0" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    warn "cannot write to $INSTALL_DIR_LOCAL or /usr/local/bin"
    INSTALL_DIR="$PWD/bin"
    mkdir -p "$INSTALL_DIR"
  fi
}

ensure_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return 0 ;;
  esac
  ADDED=0
  for f in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.profile"; do
    if [ -f "$f" ] && [ -w "$f" ]; then
      printf '\n# evolv (self-evolving AI workspace)\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$f"
      ADDED=1
      break
    fi
  done
  if [ "$ADDED" = "1" ]; then
    ok "added ${INSTALL_DIR} to your PATH (restart shell or: export PATH=\"${INSTALL_DIR}:\$PATH\")"
  else
    warn "add this to your shell rc:  export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi
}

# ── launch card ─────────────────────────────────────────────────────────────
CARD_INNER=62
card_rule() {
  i=0
  while [ "$i" -lt "$CARD_INNER" ]; do
    printf '─'
    i=$((i + 1))
  done
}
section_top() {
  label="$1"
  rest=$((CARD_INNER - ${#label} - 3))
  printf '  %s┌─ %s ' "$DIM" "$label"
  i=0
  while [ "$i" -lt "$rest" ]; do
    printf '─'
    i=$((i + 1))
  done
  printf '┐%s\n' "$RESET"
}
section_bottom() {
  printf '  %s└' "$DIM"
  card_rule
  printf '┘%s\n' "$RESET"
}
card_top() { printf '  %s╭%s╮%s\n' "$BOLD" "$(card_rule)" "$RESET"; }
card_bottom() { printf '  %s╰%s╯%s\n' "$BOLD" "$(card_rule)" "$RESET"; }
card_row() {
  # $1 = plain text (ASCII, used for width) · $2 = rendered text (may be colored)
  plain="$1"; rendered="${2:-$1}"
  pad=$((CARD_INNER - ${#plain} - 2))
  if [ "$pad" -lt 0 ]; then pad=0; fi
  printf '  %s│%s %s%*s %s│%s\n' "$BOLD" "$RESET" "$rendered" "$pad" '' "$BOLD" "$RESET"
  if [ "$UI" = "1" ]; then sleep 0.025; fi
}
card_blank() { card_row "" ""; }

launch_card() {
  ver="${INSTALLED_VERSION:-dev}"
  msg="  *  EVOLV v${ver} is installed"
  printf "\n"
  card_top
  card_blank
  card_row "$msg" "  ${GREEN}✔${RESET}  ${BOLD}EVOLV v${ver} is installed${RESET}"
  card_blank
  card_row "  \$ evolv" "  ${VIOLET}${BOLD}\$${RESET} ${CYAN}evolv${RESET}"
  card_blank
  card_row "  boots the organism at http://localhost:7373 and opens it" \
           "  boots the organism at ${CYAN}http://localhost:7373${RESET} and opens it"
  card_row "  in your browser. free models included - no API key needed." \
           "  in your browser. free models included - ${GREEN}no API key needed${RESET}."
  card_blank
  card_row "  flags   --port N | --no-open | --reset | --version" \
           "  ${DIM}flags${RESET}   --port N | --no-open | --reset | --version"
  card_row "  update  evolv update [--check]" \
           "  ${DIM}update${RESET}  evolv update [--check]"
  card_row "  grab    evolv download --dir ." \
           "  ${DIM}grab${RESET}    evolv download --dir ."
  card_row "  status  evolv status" \
           "  ${DIM}status${RESET}  evolv status"
  card_blank
  card_bottom
  printf "\n  %s\n\n" "${DIM}support: dxn1.0day@gmail.com · discord @dxn1_ai${RESET}"
}

# ── flags ───────────────────────────────────────────────────────────────────
show_version() {
  say "evolv installer v3 (docs: https://github.com/${REPO}/tree/main/docs)"
  TAG=$(latest_tag || true)
  if [ -n "$TAG" ]; then
    say "latest release: ${TAG}"
  else
    say "latest release: could not reach github.com (offline?) — install will build from source"
  fi
  say "EVOLV_VERSION env pin: ${VERSION}"
}

for arg in "$@"; do
  case "$arg" in
    --version|-v) show_version; exit 0 ;;
    --help|-h)
      say "usage: install.sh [--version] [--help]"
      say "  EVOLV_VERSION=v0.21.0 install.sh   pin a release instead of latest"
      exit 0 ;;
    EVOLV_VERSION=*) VERSION="${arg#EVOLV_VERSION=}" ;;
  esac
done

# ── main ────────────────────────────────────────────────────────────────────
main() {
  banner
  detect_platform

  if [ "$UI" = "1" ]; then
    section_top "boot sequence"
  fi
  run_stage "platform" "true" "${OS}/${ARCH}" spin
  # clear the placeholder line: run_stage already printed it

  TAG=""
  TAGFILE="${TMPDIR:-/tmp}/evolv-tag.$$"
  if [ "$VERSION" != "latest" ]; then
    case "$VERSION" in
      v*) TAG="$VERSION" ;;
      *) TAG="v$VERSION" ;;
    esac
    run_stage "release" "true" "$TAG" spin
  else
    if run_capture "release" "latest_tag" "$TAGFILE"; then
      TAG=$(cat "$TAGFILE" 2>/dev/null || true)
    fi
    if [ -z "$TAG" ]; then
      warn "could not resolve a release — source build it is"
    fi
  fi
  rm -f "$TAGFILE"

  TARGET="${BIN_NAME}_${OS}_${ARCH}"
  [ "$OS" = "windows" ] && TARGET="${TARGET}.exe"
  ARCHIVE="${BIN_NAME}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
  [ "$VERSION" = "latest" ] && ARCHIVE="${BIN_NAME}_${OS}_${ARCH}.tar.gz"

  HAVE_RELEASE=0
  if [ -n "$TAG" ] && [ "$TAG" != "https://github.com/${REPO}/releases" ]; then
    VERSION_TAG="$TAG"
    ARCHIVE="evolv_${VERSION_TAG#v}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION_TAG}/${ARCHIVE}"
    if run_stage "download" "fetch '$URL' /tmp/evolv.tar.gz" "${VERSION_TAG} · ${OS}/${ARCH}" bar soft; then
      HAVE_RELEASE=1
      INSTALLED_VERSION="${VERSION_TAG#v}"
    fi
  else
    warn "no releases published yet — building from source"
  fi

  if [ "$HAVE_RELEASE" = "1" ]; then
    choose_install_dir
    run_stage "install" "tar -xzf /tmp/evolv.tar.gz -C /tmp && ( mv /tmp/${BIN_NAME} '${INSTALL_DIR}/${BIN_NAME}' 2>/dev/null || cp /tmp/${BIN_NAME} '${INSTALL_DIR}/${BIN_NAME}' ) && chmod +x '${INSTALL_DIR}/${BIN_NAME}'" "${INSTALL_DIR}/${BIN_NAME}"
  else
    # fallback: build from source with Go
    command -v git >/dev/null 2>&1 || die "git required to build from source"
    # reuse a Go the installer already unpacked, even if it is not on PATH
    if ! command -v go >/dev/null 2>&1 && [ -x "$HOME/sdk/go/bin/go" ]; then
      export PATH="$HOME/sdk/go/bin:$PATH"
    fi
    if ! command -v go >/dev/null 2>&1; then
      warn "Go toolchain not found — installing Go 1.26 (this needs ~250MB)"
      GO_VER="1.26.0"
      case "$OS-$ARCH" in
        linux-amd64)  GO_TGZ="go${GO_VER}.linux-amd64.tar.gz" ;;
        linux-arm64)  GO_TGZ="go${GO_VER}.linux-arm64.tar.gz" ;;
        darwin-amd64) GO_TGZ="go${GO_VER}.darwin-amd64.tar.gz" ;;
        darwin-arm64) GO_TGZ="go${GO_VER}.darwin-arm64.tar.gz" ;;
        *) die "cannot auto-install Go for ${OS}-${ARCH}; install from https://go.dev/dl and re-run" ;;
      esac
      run_stage "go download" "fetch 'https://go.dev/dl/${GO_TGZ}' /tmp/go.tgz" "Go ${GO_VER}" || die "Go download failed"
      [ -d "$HOME/sdk" ] || mkdir -p "$HOME/sdk"
      # Extract into a clean directory: tar does not delete files that the new
      # release removed, and a mixed 1.24/1.26 tree fails with "redeclared".
      run_stage "go unpack" "rm -rf '$HOME/sdk/go' && tar -C '$HOME/sdk' -xzf /tmp/go.tgz" "Go ${GO_VER}" || die "could not unpack Go"
      export PATH="$HOME/sdk/go/bin:$PATH"
    fi
    choose_install_dir
    run_stage "clone" "rm -rf /tmp/evolv-src && git clone --depth 1 'https://github.com/${REPO}.git' /tmp/evolv-src" "main" || die "could not clone ${REPO}"
    run_stage "build" "cd /tmp/evolv-src && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o '${INSTALL_DIR}/${BIN_NAME}' ./cmd/evolv" "compiled ${BIN_NAME}" || die "build failed"
    INSTALLED_VERSION=$(sed -n 's/^var Version = "\(.*\)"/\1/p' /tmp/evolv-src/internal/workspace/workspace.go 2>/dev/null | head -1)
    [ -n "$INSTALLED_VERSION" ] || INSTALLED_VERSION="dev"
  fi

  chmod +x "${INSTALL_DIR}/${BIN_NAME}" 2>/dev/null || true
  VERFILE="${TMPDIR:-/tmp}/evolv-ver.$$"
  run_capture "verify" "'${INSTALL_DIR}/${BIN_NAME}' --version" "$VERFILE" || \
    warn "installed binary did not run — check ${INSTALL_DIR}/${BIN_NAME}"
  rm -f "$VERFILE"
  if [ "$UI" = "1" ]; then
    section_bottom
  fi
  ok "installed ${VIOLET}${BOLD}${INSTALL_DIR}/${BIN_NAME}${RESET}"
  ensure_path
  launch_card
}

main "$@"
