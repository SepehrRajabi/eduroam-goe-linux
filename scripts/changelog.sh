#!/usr/bin/env bash
# Print a Markdown changelog for a tag: the commits since the previous tag.
#
# Usage: scripts/changelog.sh [TAG]
#   TAG defaults to the most recent tag reachable from HEAD. If no tag
#   exists yet, everything up to HEAD is listed.
#
# Requires full history (in CI: actions/checkout with fetch-depth: 0).
set -euo pipefail

tag="${1:-$(git describe --tags --abbrev=0 2>/dev/null || true)}"
if [[ -n "$tag" ]]; then
	git rev-parse -q --verify "refs/tags/$tag" >/dev/null || {
		echo "changelog: unknown tag: $tag" >&2
		exit 1
	}
	head="$tag"
	prev="$(git describe --tags --abbrev=0 "$tag^" 2>/dev/null || true)"
else
	head="HEAD"
	prev=""
fi

if [[ -n "$prev" ]]; then
	range="$prev..$head"
	title="Changes since $prev"
else
	range="$head"
	title="Initial release"
fi

repo_url=""
if [[ -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" ]]; then
	repo_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY"
fi

echo "## $title"
echo
# Merge commits are skipped; each entry links its commit when the repo URL is known.
if [[ -n "$repo_url" ]]; then
	git log --no-merges --pretty="format:- %s ([%h]($repo_url/commit/%H))" "$range"
else
	git log --no-merges --pretty="format:- %s (%h)" "$range"
fi
echo
if [[ -n "$prev" && -n "$repo_url" ]]; then
	echo
	echo "**Full diff:** $repo_url/compare/$prev...$head"
fi
