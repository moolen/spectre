#!/bin/bash
# Script to generate all favicon versions from favicon.svg
# Generates PNG files in various sizes, ICO file, and Apple touch icon

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SVG_SOURCE="$PROJECT_ROOT/ui/public/favicon.svg"
OUTPUT_DIR="$PROJECT_ROOT/ui/public"

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Check if Inkscape is available
if ! command -v inkscape &> /dev/null; then
    error "Inkscape is not installed"
    error "Please install Inkscape to generate favicons"
    exit 1
fi

# Check if ImageMagick is available (only needed for ICO generation)
HAS_IMAGEMAGICK=false
if command -v magick &> /dev/null; then
    CONVERT_CMD="magick"
    HAS_IMAGEMAGICK=true
elif command -v convert &> /dev/null; then
    CONVERT_CMD="convert"
    HAS_IMAGEMAGICK=true
    warn "Using deprecated 'convert' command. Consider using 'magick' instead."
fi

# Check if source SVG exists
if [ ! -f "$SVG_SOURCE" ]; then
    error "Source SVG not found: $SVG_SOURCE"
    exit 1
fi

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

log "Generating favicons from: $SVG_SOURCE"
log "Output directory: $OUTPUT_DIR"

# Generate PNG files in various sizes using Inkscape (handles transparency natively)
log "Generating PNG favicons..."
inkscape --export-type=png --export-filename="$OUTPUT_DIR/favicon-16x16.png" --export-width=16 --export-height=16 "$SVG_SOURCE"
inkscape --export-type=png --export-filename="$OUTPUT_DIR/favicon-32x32.png" --export-width=32 --export-height=32 "$SVG_SOURCE"
inkscape --export-type=png --export-filename="$OUTPUT_DIR/favicon-96x96.png" --export-width=96 --export-height=96 "$SVG_SOURCE"
inkscape --export-type=png --export-filename="$OUTPUT_DIR/favicon-192x192.png" --export-width=192 --export-height=192 "$SVG_SOURCE"
inkscape --export-type=png --export-filename="$OUTPUT_DIR/favicon-512x512.png" --export-width=512 --export-height=512 "$SVG_SOURCE"

# Generate main favicon.png (using 192x192 as the main one)
log "Generating main favicon.png..."
cp "$OUTPUT_DIR/favicon-192x192.png" "$OUTPUT_DIR/favicon.png"

# Generate multi-size ICO file (preserve transparency)
# Note: Inkscape doesn't create ICO files, so we use ImageMagick if available
if [ "$HAS_IMAGEMAGICK" = true ]; then
    log "Generating favicon.ico (multi-size)..."
    $CONVERT_CMD "$OUTPUT_DIR/favicon-16x16.png" \
                "$OUTPUT_DIR/favicon-32x32.png" \
                "$OUTPUT_DIR/favicon-96x96.png" \
                "$OUTPUT_DIR/favicon-192x192.png" \
                -alpha set \
                "$OUTPUT_DIR/favicon.ico"
else
    warn "ImageMagick not found. Skipping favicon.ico generation."
    warn "Install ImageMagick to generate ICO files."
fi

# Generate Apple touch icon (180x180)
log "Generating Apple touch icon..."
inkscape --export-type=png --export-filename="$OUTPUT_DIR/apple-touch-icon.png" --export-width=180 --export-height=180 "$SVG_SOURCE"

log "Favicon generation complete!"
log "Generated files:"
log "  - favicon.svg (source, unchanged)"
log "  - favicon-16x16.png"
log "  - favicon-32x32.png"
log "  - favicon-96x96.png"
log "  - favicon-192x192.png"
log "  - favicon-512x512.png"
log "  - favicon.png (192x192)"
if [ "$HAS_IMAGEMAGICK" = true ]; then
    log "  - favicon.ico (multi-size)"
else
    log "  - favicon.ico (skipped - ImageMagick not available)"
fi
log "  - apple-touch-icon.png (180x180)"

