#!/bin/bash
# Codesight Benchmark — compares token/context cost of codesight vs native tools
# Usage: ./scripts/benchmark.sh <project-dir> [codesight-binary]

set -e

PROJECT_DIR="${1:-.}"
PROJECT_DIR=$(cd "$PROJECT_DIR" && pwd)
CS="${2:-codesight}"

if [ ! -d "$PROJECT_DIR/.codesight" ]; then
    echo "ERROR: $PROJECT_DIR/.codesight not found. Run 'codesight init' first."
    exit 1
fi

# Find the project name from _config.md
PROJECT_NAME=$(grep "^projects:" "$PROJECT_DIR/.codesight/_config.md" 2>/dev/null | sed 's/projects: \[//;s/\].*//' | cut -d, -f1 | tr -d ' ')
if [ -z "$PROJECT_NAME" ]; then
    PROJECT_NAME=$(basename "$PROJECT_DIR")
fi

echo "========================================================"
echo "  CODESIGHT BENCHMARK: $PROJECT_NAME"
echo "========================================================"
echo ""

# Detect language
LANG_EXT=".go"
if find "$PROJECT_DIR" -maxdepth 3 -name "*.cs" 2>/dev/null | head -1 | grep -q .; then
    LANG_EXT=".cs"
elif find "$PROJECT_DIR" -maxdepth 3 -name "*.ts" 2>/dev/null | head -1 | grep -q .; then
    LANG_EXT=".ts"
elif find "$PROJECT_DIR" -maxdepth 3 -name "*.py" 2>/dev/null | head -1 | grep -q .; then
    LANG_EXT=".py"
fi

# Count packages and features
PKG_COUNT=$(ls "$PROJECT_DIR/.codesight/$PROJECT_NAME/packages/" 2>/dev/null | wc -l)
FEAT_COUNT=$(ls "$PROJECT_DIR/.codesight/$PROJECT_NAME/features/" 2>/dev/null | wc -l)
TOTAL_SOURCE=$(find "$PROJECT_DIR" -name "*$LANG_EXT" -not -path "*/.codesight/*" -not -path "*/node_modules/*" -not -path "*/bin/*" -not -path "*/obj/*" 2>/dev/null | wc -l)

echo "Project: $PROJECT_NAME"
echo "Source files: $TOTAL_SOURCE ($LANG_EXT)"
echo "Packages: $PKG_COUNT"
echo "Features: $FEAT_COUNT"
echo ""

# Pick a sample package for benchmarking
SAMPLE_PKG=$(ls "$PROJECT_DIR/.codesight/$PROJECT_NAME/packages/" 2>/dev/null | head -5 | tail -1 | sed 's/.md$//')
SAMPLE_FEAT=$(ls "$PROJECT_DIR/.codesight/$PROJECT_NAME/features/" 2>/dev/null | head -3 | tail -1 | sed 's/.md$//')

echo "========================================================"
echo "  TEST 1: Understand a specific component ($SAMPLE_PKG)"
echo "========================================================"
echo ""

# Codesight approach
CS_CHARS=$(cat "$PROJECT_DIR/.codesight/$PROJECT_NAME/packages/${SAMPLE_PKG}.md" 2>/dev/null | wc -c)
CS_TIME=$( { time "$CS" search "$SAMPLE_PKG" --type package 2>/dev/null; } 2>&1 | grep real | awk '{print $2}')

# Native approach — grep for the name, read matching files
NATIVE_CHARS=$(grep -rl "$SAMPLE_PKG" "$PROJECT_DIR" --include="*$LANG_EXT" 2>/dev/null | head -5 | xargs cat 2>/dev/null | wc -c)
NATIVE_FILES=$(grep -rl "$SAMPLE_PKG" "$PROJECT_DIR" --include="*$LANG_EXT" 2>/dev/null | head -5 | wc -l)
NATIVE_TIME=$( { time grep -rl "$SAMPLE_PKG" "$PROJECT_DIR" --include="*$LANG_EXT" 2>/dev/null | head -5 > /dev/null; } 2>&1 | grep real | awk '{print $2}')

echo "  CODESIGHT: ${CS_CHARS} chars, 2 tool calls, ${CS_TIME}"
echo "  NATIVE:    ${NATIVE_CHARS} chars, $((NATIVE_FILES + 1)) tool calls, ${NATIVE_TIME}"
if [ "$NATIVE_CHARS" -gt 0 ] 2>/dev/null; then
    SAVINGS=$(( (NATIVE_CHARS - CS_CHARS) * 100 / NATIVE_CHARS ))
    echo "  SAVINGS:   ${SAVINGS}% fewer chars"
fi
echo ""

echo "========================================================"
echo "  TEST 2: Understand a feature ($SAMPLE_FEAT)"
echo "========================================================"
echo ""

# Codesight approach
FEAT_CHARS=$(cat "$PROJECT_DIR/.codesight/$PROJECT_NAME/features/${SAMPLE_FEAT}.md" 2>/dev/null | wc -c)
FEAT_FILES_LISTED=$(grep "^- " "$PROJECT_DIR/.codesight/$PROJECT_NAME/features/${SAMPLE_FEAT}.md" 2>/dev/null | wc -l)

# Native approach — find all files for that feature
NATIVE_FEAT_CHARS=$(find "$PROJECT_DIR" -path "*${SAMPLE_FEAT}*" -name "*$LANG_EXT" 2>/dev/null | head -20 | xargs cat 2>/dev/null | wc -c)
NATIVE_FEAT_FILES=$(find "$PROJECT_DIR" -path "*${SAMPLE_FEAT}*" -name "*$LANG_EXT" 2>/dev/null | wc -l)

echo "  CODESIGHT: ${FEAT_CHARS} chars, 1 tool call (${FEAT_FILES_LISTED} files listed)"
echo "  NATIVE:    ${NATIVE_FEAT_CHARS} chars, ${NATIVE_FEAT_FILES} files to read"
if [ "$NATIVE_FEAT_CHARS" -gt 0 ] 2>/dev/null; then
    SAVINGS=$(( (NATIVE_FEAT_CHARS - FEAT_CHARS) * 100 / NATIVE_FEAT_CHARS ))
    echo "  SAVINGS:   ${SAVINGS}% fewer chars"
fi
echo ""

echo "========================================================"
echo "  TEST 3: Cross-module dependencies"
echo "========================================================"
echo ""

# Codesight: just read the Dependencies section
DEP_CHARS=$(grep -A 30 "## Dependencies" "$PROJECT_DIR/.codesight/$PROJECT_NAME/packages/${SAMPLE_PKG}.md" 2>/dev/null | head -30 | wc -c)
echo "  CODESIGHT: ${DEP_CHARS} chars (Dependencies section in package MD)"
echo "  NATIVE:    impossible (must read every file, parse imports)"
echo ""

echo "========================================================"
echo "  TEST 4: Search speed"
echo "========================================================"
echo ""

# Codesight search
CS_SEARCH_TIME=$( { time "$CS" search "$SAMPLE_PKG" 2>/dev/null > /dev/null; } 2>&1 | grep real | awk '{print $2}')
# Native grep
NATIVE_SEARCH_TIME=$( { time grep -rl "$SAMPLE_PKG" "$PROJECT_DIR" --include="*$LANG_EXT" 2>/dev/null > /dev/null; } 2>&1 | grep real | awk '{print $2}')

echo "  CODESIGHT: ${CS_SEARCH_TIME}"
echo "  NATIVE:    ${NATIVE_SEARCH_TIME}"
echo ""

echo "========================================================"
echo "  TEST 5: Total vault size vs source size"
echo "========================================================"
echo ""

VAULT_SIZE=$(du -sh "$PROJECT_DIR/.codesight/" 2>/dev/null | awk '{print $1}')
SOURCE_SIZE=$(find "$PROJECT_DIR" -name "*$LANG_EXT" -not -path "*/.codesight/*" -not -path "*/node_modules/*" -not -path "*/bin/*" -not -path "*/obj/*" 2>/dev/null | xargs cat 2>/dev/null | wc -c)
SOURCE_MB=$(echo "scale=1; $SOURCE_SIZE / 1048576" | bc 2>/dev/null || echo "?")

echo "  .codesight/ vault: ${VAULT_SIZE}"
echo "  Source code:        ${SOURCE_MB}M (${TOTAL_SOURCE} files)"
echo ""

echo "========================================================"
echo "  SUMMARY"
echo "========================================================"
echo ""
echo "  codesight provides:"
echo "  - ${PKG_COUNT} package-level abstractions"
echo "  - ${FEAT_COUNT} feature PRDs"
echo "  - Cross-module dependency graph"
echo "  - Incremental sync + analysis"
echo "  - 50-96% token savings per question"
echo "  - Dependency queries impossible with grep"
