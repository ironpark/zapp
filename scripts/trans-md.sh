#!/bin/bash

#==============================================================================
# Markdown Translation Script
#==============================================================================
# Purpose: Translate markdown files to specified languages using OpenRouter API
#
# Required Environment Variables:
#   OPENROUTER_API_KEY    - OpenRouter API key (required)
#   OPENROUTER_URL        - OpenRouter API endpoint (default: https://openrouter.ai/api/v1/chat/completions)
#   OPENROUTER_MODEL      - Model to use (default: anthropic/claude-3.5-sonnet)
#   REFERER_URL           - HTTP Referer header (default: https://github.com/markdown-translator)
#   APP_TITLE             - X-Title header (default: Markdown Translator)
#
# Usage:
#   ./translate_md.sh <input_file> <target_language>
#
# Examples:
#   ./translate_md.sh readme.md ja
#   ./translate_md.sh docs/guide.md ko
#
#
# Output:
#   original_filename.language_code.md (e.g., readme.ja.md, guide.ko.md)
#==============================================================================

# Load .env file if it exists
if [ -f ".env" ]; then
    source .env
fi

# Environment variable setup (with defaults)
OPENROUTER_API_KEY="${OPENROUTER_API_KEY:-}"
OPENROUTER_URL="${OPENROUTER_URL:-https://openrouter.ai/api/v1/chat/completions}"
OPENROUTER_MODEL="${OPENROUTER_MODEL:-anthropic/claude-3.5-sonnet}"
REFERER_URL="${REFERER_URL:-https://github.com/markdown-translator}"
APP_TITLE="${APP_TITLE:-Markdown Translator}"

# Check required variables
if [ -z "$OPENROUTER_API_KEY" ]; then
    echo "Error: OPENROUTER_API_KEY environment variable is not set."
    echo "  Add it to .env file or run: export OPENROUTER_API_KEY='your-key'"
    exit 1
fi

# Check arguments
if [ $# -ne 2 ]; then
    echo "Error: Invalid arguments."
    echo "Usage: $0 <input_file> <target_language>"
    exit 1
fi

INPUT_FILE="$1"
TARGET_LANG="$2"

# Check file existence
if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File '$INPUT_FILE' not found."
    exit 1
fi

# Check bash version and use appropriate array syntax
if [[ ${BASH_VERSION%%.*} -ge 4 ]]; then
    # Bash 4+ supports associative arrays
    declare -A LANG_MODELS=(
        ["en"]="meta-llama/llama-3.3-70b-instruct:free"
        ["ko"]="google/gemini-2.0-flash-exp:free"
        ["ja"]="google/gemini-2.0-flash-exp:free"
        ["zh"]="qwen/qwen3-30b-a3b:free"
        ["es"]="meta-llama/llama-3.3-70b-instruct:free"
        ["fr"]="meta-llama/llama-3.3-70b-instruct:free"
        ["de"]="meta-llama/llama-3.3-70b-instruct:free"
    )

    declare -A LANG_NAMES=(
        ["ja"]="Japanese"
        ["ko"]="Korean"
        ["zh"]="Chinese"
        ["es"]="Spanish"
        ["fr"]="French"
        ["de"]="German"
        ["en"]="English"
    )
else
    # Fallback for older bash versions
    echo "Warning: Bash version ${BASH_VERSION} detected. Using fallback model selection."
fi

# Default fallback model
DEFAULT_MODEL="meta-llama/llama-3.3-70b-instruct:free"

# Get language name and model (with fallback for older bash)
get_lang_info() {
    local lang="$1"
    case $lang in
        ja) echo "Japanese|google/gemini-2.0-flash-exp:free" ;;
        ko) echo "Korean|google/gemini-2.0-flash-exp:free" ;;
        zh) echo "Chinese|meta-llama/llama-3.3-70b-instruct:free" ;;
        es) echo "Spanish|meta-llama/llama-3.3-70b-instruct:free" ;;
        fr) echo "French|meta-llama/llama-3.3-70b-instruct:free" ;;
        de) echo "German|meta-llama/llama-3.3-70b-instruct:free" ;;
        en) echo "English|meta-llama/llama-3.3-70b-instruct:free" ;;
        *) echo "Unknown|$DEFAULT_MODEL" ;;
    esac
}

# Check if language is supported and get language name and model
if [[ ${BASH_VERSION%%.*} -ge 4 ]] && [[ ${LANG_NAMES[$TARGET_LANG]+_} ]]; then
    # Use associative arrays if available
    LANG_NAME="${LANG_NAMES[$TARGET_LANG]}"
    if [[ ${LANG_MODELS[$TARGET_LANG]+_} ]]; then
        SELECTED_MODEL="${LANG_MODELS[$TARGET_LANG]}"
    else
        SELECTED_MODEL="$DEFAULT_MODEL"
        echo "Warning: No specific model configured for $TARGET_LANG, using default: $DEFAULT_MODEL"
    fi
else
    # Use function fallback
    lang_info=$(get_lang_info "$TARGET_LANG")
    LANG_NAME="${lang_info%|*}"
    SELECTED_MODEL="${lang_info#*|}"

    if [[ "$LANG_NAME" == "Unknown" ]]; then
        echo "Unsupported language code: $TARGET_LANG"
        echo "Supported languages: en ja ko zh es fr de"
        exit 1
    fi
fi

# Override the model for this specific language
OPENROUTER_MODEL="$SELECTED_MODEL"

# Generate output filename
FILENAME=$(basename "$INPUT_FILE" .md)
DIRNAME=$(dirname "$INPUT_FILE")
OUTPUT_FILE="$DIRNAME/$FILENAME.$TARGET_LANG.md"

echo "Starting translation: $INPUT_FILE -> $OUTPUT_FILE ($LANG_NAME)"
echo "Using model: $SELECTED_MODEL"

# Read markdown content
CONTENT=$(cat "$INPUT_FILE")

# JSON escape function (fixed for cross-platform compatibility)
json_escape() {
    local input="$1"
    # Use a more compatible approach
    printf '%s' "$input" | sed 's/\\/\\\\/g; s/"/\\"/g; s/$/\\n/; $s/\\n$//' | tr -d '\n' | sed 's/\r/\\r/g; s/\t/\\t/g'
}

# Escape content
ESCAPED_CONTENT=$(json_escape "$CONTENT")

# Generate API request JSON
JSON_PAYLOAD=$(cat << EOF
{
  "model": "$OPENROUTER_MODEL",
  "messages": [
    {
      "role": "system",
      "content": "You are a professional translator specializing in technical documentation. Translate the provided markdown content to $LANG_NAME following these strict guidelines:\\n\\n1. ALWAYS translate the actual content provided by the user\\n2. Preserve all markdown formatting (headers, links, code blocks, lists, etc.)\\n3. Do NOT translate code syntax, variable names, function names, or keywords\\n4. DO translate comments within code blocks\\n5. Keep URLs, file paths, and technical identifiers unchanged\\n6. Use natural, fluent $LANG_NAME that sounds native\\n7. Consider cultural and technical conventions for $LANG_NAME\\n\\nIMPORTANT: Translate the markdown content directly. Do not ask for content or provide explanations - just translate what is given."
    },
    {
      "role": "user",
      "content": "$ESCAPED_CONTENT"
    }
  ],
  "temperature": 0.1,
  "max_tokens": 8000
}
EOF
)

# API call and response handling
echo "Calling OpenRouter API..."
RESPONSE=$(curl -s -X POST "$OPENROUTER_URL" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "HTTP-Referer: $REFERER_URL" \
  -H "X-Title: $APP_TITLE" \
  -d "$JSON_PAYLOAD")

# Check response
if [ $? -ne 0 ]; then
    echo "Error: API call failed"
    exit 1
fi

# Extract translated text from JSON response
TRANSLATED_CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content' 2>/dev/null)

# Check jq execution and API response
if [ $? -ne 0 ] || [ "$TRANSLATED_CONTENT" = "null" ] || [ -z "$TRANSLATED_CONTENT" ]; then
    echo "Error: Failed to parse API response or empty response"
    echo "Response: $RESPONSE"

    # Check for specific API errors
    error_msg=$(echo "$RESPONSE" | jq -r '.error.message' 2>/dev/null)
    if [ "$error_msg" != "null" ] && [ -n "$error_msg" ]; then
        echo "API Error: $error_msg"
    fi
    exit 1
fi

# Check if the response is just asking for content (common AI response issue)
if echo "$TRANSLATED_CONTENT" | grep -qi "provide\|내용을\|コンテンツ\|请提供"; then
    echo "Error: Model returned a request for content instead of translation"
    echo "This might be due to JSON formatting issues or model limitations"
    echo "Response preview: $(echo "$TRANSLATED_CONTENT" | head -c 200)..."
    exit 1
fi

# Save translated content to file
echo "$TRANSLATED_CONTENT" > "$OUTPUT_FILE"

echo "Translation completed: $OUTPUT_FILE"
echo "Translated file size: $(wc -c < "$OUTPUT_FILE") bytes"

# Preview (first 5 lines)
echo ""
echo "=== Translation Preview ==="
head -n 5 "$OUTPUT_FILE"
echo "..."