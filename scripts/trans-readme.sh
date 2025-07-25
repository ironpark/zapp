#!/bin/bash

#==============================================================================
# README Translation Script
#==============================================================================
# Purpose: Translate README.md to multiple languages (English, Korean, Japanese, Chinese)
#
# Required Environment Variables:
#   OPENROUTER_API_KEY    - OpenRouter API key (required)
#
# Usage:
#   ./translate_readme.sh [readme_file]
#
# Examples:
#   ./translate_readme.sh                    # Translates ./README.md
#   ./translate_readme.sh ./docs/README.md   # Translates specific README
#
# Output Files:
#   README.en.md (English)
#   README.ko.md (Korean)
#   README.ja.md (Japanese)
#   README.zh.md (Chinese)
#==============================================================================

# Load .env file if it exists
if [ -f ".env" ]; then
    source .env
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TRANSLATOR_SCRIPT="$SCRIPT_DIR/trans-md-batch.sh"

# Default README file
DEFAULT_README="README.md"

# Target languages for README
README_LANGUAGES="ko ja"

# Language descriptions
declare -A LANG_DESCRIPTIONS=(
    ["en"]="English"
    ["ko"]="Korean (한국어)"
    ["ja"]="Japanese (日本語)"
    ["zh"]="Chinese (中文)"
)

# Check if translator script exists
if [ ! -f "$TRANSLATOR_SCRIPT" ]; then
    echo "Error: Translation script '$TRANSLATOR_SCRIPT' not found."
    echo "Make sure 'translate_md.sh' is in the same directory."
    exit 1
fi

# Determine README file
if [ $# -eq 0 ]; then
    README_FILE="$DEFAULT_README"
else
    README_FILE="$1"
fi

# Check if README file exists
if [ ! -f "$README_FILE" ]; then
    echo "Error: README file '$README_FILE' not found."
    exit 1
fi

echo "🌍 README Multi-Language Translation"
echo "===================================="
echo "Source file: $README_FILE"
echo "Target languages: English, Korean, Japanese, Chinese"
echo ""

# Check for existing translations
existing_files=""
for lang in $README_LANGUAGES; do
    filename=$(basename "$README_FILE" .md)
    dirname=$(dirname "$README_FILE")
    target_file="$dirname/$filename.$lang.md"

    if [ -f "$target_file" ]; then
        existing_files="$existing_files $target_file"
    fi
done

if [ -n "$existing_files" ]; then
    echo "⚠️  Warning: The following translation files already exist:"
    for file in $existing_files; do
        echo "    $file"
    done
    echo ""
    read -p "Overwrite existing files? (y/N): " confirm
    if [[ $confirm != [yY] ]]; then
        echo "Translation cancelled."
        exit 0
    fi
    echo ""
fi

# Start translation process
echo "🚀 Starting translation process..."
echo ""

success_count=0
fail_count=0
start_time=$(date +%s)

for lang in $README_LANGUAGES; do
    lang_desc="${LANG_DESCRIPTIONS[$lang]}"
    echo "📝 Translating to $lang_desc..."

    if bash "$TRANSLATOR_SCRIPT" "$README_FILE" "$lang"; then
        ((success_count++))
        echo "✅ Success: README.$lang.md created"
    else
        ((fail_count++))
        echo "❌ Failed: $lang_desc translation"
    fi

    echo "   ⏳ Waiting 2 seconds before next translation..."
    sleep 2
    echo ""
done

# Summary
end_time=$(date +%s)
duration=$((end_time - start_time))

echo "🎉 Translation Complete!"
echo "======================="
echo "✅ Successful: $success_count languages"
echo "❌ Failed: $fail_count languages"
echo "⏱️  Total time: ${duration} seconds"
echo ""

if [ $success_count -gt 0 ]; then
    echo "📁 Generated files:"
    filename=$(basename "$README_FILE" .md)
    dirname=$(dirname "$README_FILE")

    for lang in $README_LANGUAGES; do
        target_file="$dirname/$filename.$lang.md"
        if [ -f "$target_file" ]; then
            file_size=$(wc -c < "$target_file")
            echo "    📄 $target_file ($file_size bytes)"
        fi
    done
    echo ""
    echo "💡 Tip: Add these files to your repository to support international users!"
fi