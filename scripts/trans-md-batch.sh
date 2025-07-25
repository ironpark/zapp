#!/bin/bash

#==============================================================================
# Batch Markdown Translation Script
#==============================================================================
# Purpose: Translate markdown files to multiple languages using OpenRouter API
#
# Required Environment Variables:
#   OPENROUTER_API_KEY    - OpenRouter API key (required)
#   OPENROUTER_URL        - OpenRouter API endpoint (optional)
#   OPENROUTER_MODEL      - Model to use (optional)
#
# Usage:
#   ./batch_translate.sh <input_file_or_directory> [language_codes...]
#
# Examples:
#   ./batch_translate.sh readme.md en ko ja zh        # Single file to multiple languages
#   ./batch_translate.sh ./docs ko ja                 # All .md files in directory to specific languages
#   ./batch_translate.sh readme.md                    # Single file to default languages (en,ko,ja,zh)
#
# Supported Languages:
#   en (English), ja (Japanese), ko (Korean), zh (Chinese), es (Spanish), fr (French), de (German)
#
# Notes:
#   - Skips files that already have language suffix (e.g., readme.ko.md)
#   - Adds 2-second delay between API calls to avoid rate limiting
#==============================================================================

# Load .env file if it exists
if [ -f ".env" ]; then
    source .env
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TRANSLATOR_SCRIPT="$SCRIPT_DIR/trans-md.sh"

# Default languages if none specified
DEFAULT_LANGUAGES="ko ja"

# Language descriptions for display
get_language_name() {
    case "$1" in
        en) echo "English" ;;
        ko) echo "Korean (한국어)" ;;
        ja) echo "Japanese (日本語)" ;;
        zh) echo "Chinese (中文)" ;;
        es) echo "Spanish (Español)" ;;
        fr) echo "French (Français)" ;;
        de) echo "German (Deutsch)" ;;
        *) echo "Unknown" ;;
    esac
}

# Check if translator script exists
if [ ! -f "$TRANSLATOR_SCRIPT" ]; then
    echo "Error: Translation script '$TRANSLATOR_SCRIPT' not found."
    exit 1
fi

# Check arguments
if [ $# -eq 0 ]; then
    echo "Error: No input specified."
    echo "Usage: $0 <input_file_or_directory> [language_codes...]"
    exit 1
fi

INPUT="$1"
shift

# Determine target languages
if [ $# -eq 0 ]; then
    LANGUAGES="$DEFAULT_LANGUAGES"
    echo "No languages specified. Using default: $LANGUAGES"
else
    LANGUAGES="$*"
fi

echo "Target languages: $LANGUAGES"

# Function to translate a single file to multiple languages
translate_file() {
    local file="$1"
    local success_count=0
    local fail_count=0
    local success_langs=""
    local failed_langs=""

    echo ""
    echo "=========================================="
    echo "Translating file: $file"
    echo "=========================================="

    for lang in $LANGUAGES; do
        # Skip if target file already exists
        local filename=$(basename "$file" .md)
        local dirname=$(dirname "$file")
        local target_file="$dirname/$filename.$lang.md"

        if [ -f "$target_file" ]; then
            echo "⚠️  Skipping $lang: $target_file already exists"
            continue
        fi

        echo "🔄 Translating to $lang..."

        if bash "$TRANSLATOR_SCRIPT" "$file" "$lang"; then
            ((success_count++))
            success_langs="$success_langs $lang"
            echo "✅ Success: $target_file"
        else
            ((fail_count++))
            failed_langs="$failed_langs $lang"
            echo "❌ Failed: $lang translation"
        fi

        # Rate limiting delay
        echo "   Waiting 2 seconds..."
        sleep 2
    done

    echo ""
    echo "File translation summary:"
    echo "  Success: $success_count languages ($success_langs)"
    echo "  Failed: $fail_count languages ($failed_langs)"

    # Return info for global counting
    echo "$success_count:$fail_count:$success_langs:$failed_langs"
}

# Main logic
if [ -f "$INPUT" ]; then
    # Single file translation
    if [[ "$INPUT" =~ \.[a-z]{2}\.md$ ]]; then
        echo "Warning: Input file appears to be already translated (has language suffix)"
        echo "File: $INPUT"
        read -p "Continue anyway? (y/N): " confirm
        if [[ $confirm != [yY] ]]; then
            echo "Translation cancelled."
            exit 0
        fi
    fi

    result=$(translate_file "$INPUT")
    IFS=':' read -r success_count fail_count success_langs fail_langs <<< "$result"

    echo ""
    echo "🎉 Translation Complete!"
    echo "======================="
    echo "✅ Successful: $success_count languages"
    echo "❌ Failed: $fail_count languages"

    if [ $success_count -gt 0 ]; then
        echo ""
        echo "📁 Generated files:"
        filename=$(basename "$INPUT" .md)
        dirname=$(dirname "$INPUT")

        for lang in $success_langs; do
            target_file="$dirname/$filename.$lang.md"
            if [ -f "$target_file" ]; then
                file_size=$(wc -c < "$target_file")
                lang_name=$(get_language_name "$lang")
                echo "    📄 $target_file ($file_size bytes) - $lang_name"
            fi
        done
    fi

elif [ -d "$INPUT" ]; then
    # Directory translation
    echo "Searching for markdown files in: $INPUT"

    # Find all .md files, excluding those with language suffixes
    MD_FILES=$(find "$INPUT" -name "*.md" -type f | grep -v '\.[a-z][a-z]\.md$')

    if [ -z "$MD_FILES" ]; then
        echo "No translatable markdown files found in directory."
        exit 1
    fi

    echo "Found files:"
    echo "$MD_FILES" | nl -v 0

    file_count=$(echo "$MD_FILES" | wc -l)
    lang_count=$(echo $LANGUAGES | wc -w)
    total_translations=$((file_count * lang_count))

    echo ""
    echo "Translation plan:"
    echo "  Files: $file_count"
    echo "  Languages: $lang_count ($LANGUAGES)"
    echo "  Total translations: $total_translations"
    echo ""

    read -p "Proceed with batch translation? (y/N): " confirm

    if [[ $confirm != [yY] ]]; then
        echo "Translation cancelled."
        exit 0
    fi

    # Translate each file
    total_success=0
    total_fail=0
    all_generated_files=""

    while IFS= read -r file; do
        result=$(translate_file "$file")
        IFS=':' read -r success_count fail_count success_langs fail_langs <<< "$result"

        total_success=$((total_success + success_count))
        total_fail=$((total_fail + fail_count))

        # Collect generated files
        if [ "${success_count:-0}" -gt 0 ]; then
            filename=$(basename "$file" .md)
            dirname=$(dirname "$file")
            for lang in $success_langs; do
                if [ -n "$lang" ]; then
                    all_generated_files="$all_generated_files $dirname/$filename.$lang.md:$lang"
                fi
            done
        fi
    done <<< "$MD_FILES"

    echo ""
    echo "=========================================="
    echo "BATCH TRANSLATION COMPLETE"
    echo "=========================================="
    echo "Total successful translations: $total_success"
    echo "Total failed translations: $total_fail"

    if [ -n "$all_generated_files" ]; then
        echo ""
        echo "📁 All generated files:"
        for file_info in $all_generated_files; do
            file_path="${file_info%:*}"
            lang="${file_info#*:}"
            if [ -f "$file_path" ]; then
                file_size=$(wc -c < "$file_path")
                lang_name=$(get_language_name "$lang")
                echo "    📄 $file_path ($file_size bytes) - $lang_name"
            fi
        done
    fi

else
    echo "Error: '$INPUT' is neither a file nor a directory."
    exit 1
fi