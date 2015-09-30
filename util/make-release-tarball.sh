#!/bin/bash
#
# This script creates a release tarball for Holo, including the required
# submodules (Github sadly does not include these in its autogenerated
# release tarball, so these are useless for compiling Holo).

# exit on error
set -e

cd "$( git rev-parse --show-toplevel )"
REPO_ROOT="$PWD"

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <commit-ish> <archive-name>" >&2
    echo 'where <archive-name> looks like "holo-0.6.0"'
    exit 1
fi
REVISION=$( git rev-parse --verify "$1" || exit 1 )
TOPLEVELDIR="$2"

# create a temp directory
rm -rf -- "build/releases/$TOPLEVELDIR"
mkdir -p  "build/releases/$TOPLEVELDIR"

# dump the specified revision of the repository into there
git archive $REVISION | ( cd "build/releases/$TOPLEVELDIR"; tar xf - )

# for each submodule in that commit...
perl -nE '/^\s*path\s*=\s*(.+?)\s*$/ && say $1' "build/releases/$TOPLEVELDIR/.gitmodules" | while read SUBMODULE_PATH; do
    # find the submodule revision for the current root revision
    SUBMODULE_REV=$( git ls-tree $REVISION "$SUBMODULE_PATH" | awk '{print $3}' )
    # and dump that revision of the submodule into our release tree as well
    git -C "$REPO_ROOT/$SUBMODULE_PATH" archive $SUBMODULE_REV | ( cd "build/releases/$TOPLEVELDIR/$SUBMODULE_PATH"; tar xf - )
    # NOTE: Turning this loop into a recursive function to deal with recursive
    # submodules is left as an exercise to the reader, if (and only if) the
    # need ever arises.
done

# compress into tarball
( cd build/releases/; tar czf "${TOPLEVELDIR}.tar.gz" "$TOPLEVELDIR" )
