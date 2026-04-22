#!/bin/sh
#
# contrib/clip-notify-test.sh — self-contained tests for the json_escape
# function inside contrib/clip-notify. Runs in-process and does not require
# a clipper daemon or network connectivity.
#
# Exits 0 iff every test passed.

set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SRC=$HERE/clip-notify

if [ ! -r "$SRC" ]; then
    printf >&2 'clip-notify-test.sh: cannot find clip-notify at %s\n' "$SRC"
    exit 2
fi

# Extract the json_escape() function body from clip-notify and eval it into
# the current shell. This lets us exercise the function directly without
# spinning up nc or needing a clipper instance to be running. The awk pattern
# relies on json_escape() being the only function in the file whose opening
# line starts at column 0 with `json_escape() {` and whose closing brace is
# the only `}` anchored at column 0 after that point.
eval "$(
    awk '
        /^json_escape\(\) \{/ { inside = 1; print; next }
        inside                { print }
        inside && /^\}$/      { exit }
    ' "$SRC"
)"

pass=0
fail=0

check_valid() {
    # check_valid DESCRIPTION INPUT EXPECTED_OUTPUT
    _desc=$1
    _input=$2
    _expected=$3
    if ! _got=$(json_escape "$_input" 2>/dev/null); then
        printf '[FAIL] %s: rejected valid input\n' "$_desc"
        fail=$((fail + 1))
        return
    fi
    if [ "$_got" = "$_expected" ]; then
        printf '[PASS] %s\n' "$_desc"
        pass=$((pass + 1))
    else
        printf '[FAIL] %s\n       expected: %s\n       got:      %s\n' \
            "$_desc" "$_expected" "$_got"
        fail=$((fail + 1))
    fi
}

check_invalid() {
    # check_invalid DESCRIPTION INPUT
    _desc=$1
    _input=$2
    if json_escape "$_input" >/dev/null 2>&1; then
        printf '[FAIL] %s: accepted invalid input\n' "$_desc"
        fail=$((fail + 1))
    else
        printf '[PASS] %s (rejected)\n' "$_desc"
        pass=$((pass + 1))
    fi
}

printf '=== valid inputs ===\n'

check_valid "ASCII"                  "hello world"                      "hello world"
check_valid "Latin-1 accented"       "Héllo wörld"                      "Héllo wörld"
check_valid "CJK (3-byte UTF-8)"     "日本語"                           "日本語"
check_valid "Emoji (4-byte UTF-8)"   "🎉🚀"                             "🎉🚀"
check_valid "Backslash"              'back\slash'                       'back\\slash'
check_valid "Double quote"           'with "quote"'                     'with \"quote\"'
check_valid "Short-form controls"    "$(printf 'a\tb\nc\rd\be\ff')"     'a\tb\nc\rd\be\ff'
check_valid "Generic \\u00XX escape" "$(printf 'x\001y\037z')"          'x\u0001y\u001fz'
check_valid "Mixed ASCII + Unicode"  "hi 🎉, 日本!"                     "hi 🎉, 日本!"
check_valid "Empty string"           ""                                 ""

printf '\n=== invalid UTF-8 ===\n'

check_invalid "stray continuation byte (0x80)" "$(printf 'a\x80b')"
check_invalid "overlong 2-byte (0xC0 0xAF)"    "$(printf '\xc0\xaf')"
check_invalid "overlong 3-byte (0xE0 0x80..)"  "$(printf '\xe0\x80\xaf')"
check_invalid "UTF-16 surrogate (0xED 0xA0..)" "$(printf '\xed\xa0\x80')"
check_invalid "above U+10FFFF (0xF4 0x90..)"   "$(printf '\xf4\x90\x80\x80')"
check_invalid "unassigned start byte (0xF5)"   "$(printf '\xf5\x80\x80\x80')"
check_invalid "unassigned start byte (0xFF)"   "$(printf '\xff')"
check_invalid "truncated 2-byte"               "$(printf '\xc2')"
check_invalid "truncated 3-byte"               "$(printf '\xe0\xa0')"
check_invalid "truncated 4-byte"               "$(printf '\xf0\x9f\x8e')"

printf '\n%d passed, %d failed.\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
