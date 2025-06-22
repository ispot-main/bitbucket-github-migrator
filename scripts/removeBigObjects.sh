#!/bin/bash
# thanks to https://stackoverflow.com/a/17890278
# download BFG repo cleaner and put it at ~/Downloads/bfg.jar for this to work
set -euo pipefail
java -jar ~/Downloads/bfg.jar --strip-blobs-bigger-than 100M "$1"
git reflog expire --expire=now --all
git gc --prune=now --aggressive