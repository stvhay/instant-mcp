#!/usr/bin/env bash
set -euo pipefail

# Build .skill packages from .claude/skills/ directories
# Usage: build-skills.sh [skill-name...]

readonly SKILLS_DIR=".claude/skills"
readonly DIST_DIR="dist"
readonly CHECKSUMS_FILE="$DIST_DIR/.skill-checksums"

# Counters
built=0
skipped=0
orphans=0

# Colors (disabled if not a terminal)
if [[ -t 1 ]]; then
    GREEN=$'\033[0;32m'
    YELLOW=$'\033[0;33m'
    RED=$'\033[0;31m'
    BOLD=$'\033[1m'
    RESET=$'\033[0m'
else
    GREEN='' YELLOW='' RED='' BOLD='' RESET=''
fi

die() {
    printf "${RED}✗ %s${RESET}\n" "$*" >&2
    exit 1
}

# Check dependencies
command -v zip >/dev/null 2>&1 || die "Error: zip command not found"

# Compute content hash for a skill directory
compute_hash() {
    local skill_dir="$1"
    find "$skill_dir" -type f -print0 \
        | sort -z \
        | xargs -0 sha256sum 2>/dev/null \
        | sha256sum \
        | cut -d' ' -f1
}

# Get stored hash from checksums file
get_stored_hash() {
    local skill_name="$1"
    if [[ -f "$CHECKSUMS_FILE" ]]; then
        grep "^${skill_name}:" "$CHECKSUMS_FILE" 2>/dev/null | cut -d: -f2 || true
    fi
}

# Update hash in checksums file
update_hash() {
    local skill_name="$1"
    local hash="$2"

    if [[ -f "$CHECKSUMS_FILE" ]]; then
        # Remove old entry if exists
        grep -v "^${skill_name}:" "$CHECKSUMS_FILE" > "$CHECKSUMS_FILE.tmp" || true
        mv "$CHECKSUMS_FILE.tmp" "$CHECKSUMS_FILE"
    fi

    # Add new entry
    printf '%s:%s\n' "$skill_name" "$hash" >> "$CHECKSUMS_FILE"
}

# Remove hash from checksums file
remove_hash() {
    local skill_name="$1"

    if [[ -f "$CHECKSUMS_FILE" ]]; then
        grep -v "^${skill_name}:" "$CHECKSUMS_FILE" > "$CHECKSUMS_FILE.tmp" || true
        mv "$CHECKSUMS_FILE.tmp" "$CHECKSUMS_FILE"
    fi
}

# Build a single skill
build_skill() {
    local skill_name="$1"
    local skill_dir="$SKILLS_DIR/$skill_name"
    local output_file="$DIST_DIR/${skill_name}.skill"

    if [[ ! -d "$skill_dir" ]]; then
        die "Error: skill directory not found: $skill_dir"
    fi

    local current_hash stored_hash
    current_hash=$(compute_hash "$skill_dir")
    stored_hash=$(get_stored_hash "$skill_name")

    if [[ "$current_hash" == "$stored_hash" ]] && [[ -f "$output_file" ]]; then
        printf "${YELLOW}○ Skipped:${RESET} %s (unchanged)\n" "$skill_name"
        ((skipped++)) || true
        return
    fi

    # Build the package
    (cd "$skill_dir" && zip -qr - .) > "$output_file"
    update_hash "$skill_name" "$current_hash"

    printf "${GREEN}✓ Built:${RESET} %s\n" "$skill_name"
    ((built++)) || true
}

# Check for orphaned .skill files
check_orphans() {
    local skill_file skill_name

    for skill_file in "$DIST_DIR"/*.skill; do
        # Handle case where no .skill files exist
        [[ -e "$skill_file" ]] || continue

        skill_name="${skill_file##*/}"
        skill_name="${skill_name%.skill}"

        if [[ ! -d "$SKILLS_DIR/$skill_name" ]]; then
            ((orphans++)) || true
            printf "${RED}⚠ Orphaned:${RESET} ${skill_name}.skill - "
            read -rp "delete? [y/N] " answer
            if [[ "$answer" =~ ^[Yy]$ ]]; then
                rm "$skill_file"
                remove_hash "$skill_name"
                printf "${RED}✗ Deleted:${RESET} %s.skill\n" "$skill_name"
            fi
        fi
    done
}

# Get list of skills to build
get_skills() {
    if [[ $# -gt 0 ]]; then
        # Use provided skill names
        printf '%s\n' "$@"
    else
        # List all skill directories
        for dir in "$SKILLS_DIR"/*/; do
            [[ -d "$dir" ]] || continue
            basename "$dir"
        done
    fi
}

main() {
    # Ensure dist directory exists
    mkdir -p "$DIST_DIR"

    # Ensure checksums file exists
    touch "$CHECKSUMS_FILE"

    # Build skills
    local skill
    while IFS= read -r skill; do
        build_skill "$skill"
    done < <(get_skills "$@")

    # Check for orphans
    check_orphans

    # Print summary
    printf "\n${BOLD}Summary:${RESET} ${GREEN}%d built${RESET}, ${YELLOW}%d skipped${RESET}, ${RED}%d orphans${RESET}\n" "$built" "$skipped" "$orphans"
}

main "$@"
