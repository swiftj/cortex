#!/usr/bin/env bash
#
# Bump the project version following SemVer.
# Usage: ./scripts/bump-version.sh [major|minor|patch]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$ROOT_DIR/VERSION"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: $0 [major|minor|patch]"
    echo ""
    echo "Bumps the project version following SemVer:"
    echo "  major  - Breaking changes (1.0.0 -> 2.0.0)"
    echo "  minor  - New features (1.0.0 -> 1.1.0)"
    echo "  patch  - Bug fixes (1.0.0 -> 1.0.1)"
    exit 1
}

if [[ $# -ne 1 ]]; then
    usage
fi

BUMP_TYPE="$1"

if [[ ! "$BUMP_TYPE" =~ ^(major|minor|patch)$ ]]; then
    echo -e "${RED}Error: Invalid bump type '$BUMP_TYPE'${NC}"
    usage
fi

# Read current version
if [[ ! -f "$VERSION_FILE" ]]; then
    echo -e "${RED}Error: VERSION file not found at $VERSION_FILE${NC}"
    exit 1
fi

CURRENT_VERSION=$(cat "$VERSION_FILE" | tr -d '\n')
echo -e "${YELLOW}Current version: $CURRENT_VERSION${NC}"

# Parse semver
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Validate parsed values
if [[ -z "$MAJOR" || -z "$MINOR" || -z "$PATCH" ]]; then
    echo -e "${RED}Error: Invalid version format in VERSION file${NC}"
    exit 1
fi

# Calculate new version
case $BUMP_TYPE in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"

# Update VERSION file
echo "$NEW_VERSION" > "$VERSION_FILE"
echo -e "${GREEN}New version: $NEW_VERSION${NC}"

# Optionally create git tag
read -p "Create git tag v$NEW_VERSION? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git add "$VERSION_FILE"
    git commit -m "chore: bump version to $NEW_VERSION"
    git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
    echo -e "${GREEN}Created tag v$NEW_VERSION${NC}"
    echo -e "${YELLOW}Don't forget to push: git push origin main --tags${NC}"
fi
