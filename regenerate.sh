#!/bin/sh

set -euo pipefail

if [ ! -d "third_party/codemirror/node_modules" ]
then
    echo "Cannot re-generate CodeMirror JavaScript bundle:" >&2
    echo "" >&2
    echo "	No node_modules directory found in third_party/codemirror!" >&2
    echo "" >&2
    echo "To install the CodeMirror editor and dependencies into third_party," >&2
    echo "install the npm JavaScript package manager and run:" >&2
    echo "" >&2
    echo "	(cd third_party/codemirror && npm install)" >&2
    echo "" >&2
    exit 2
fi

if [ ! -d "third_party/mermaid/node_modules" ]
then
    echo "Cannot re-generate Mermaid JavaScript bundle:" >&2
    echo "" >&2
    echo "	No node_modules directory found in third_party/mermaid!" >&2
    echo "" >&2
    echo "To install the Mermaid diagram library into third_party," >&2
    echo "install the npm JavaScript package manager and run:" >&2
    echo "" >&2
    echo "	(cd third_party/mermaid && npm install)" >&2
    echo "" >&2
    exit 2
fi

# Run tsc (TypeScript compiler) for type-checking
tsc --noEmit

# Bundle npm dependencies (and TypeScript code)
export NODE_PATH=$PWD/third_party/codemirror/node_modules
go tool esbuild \
   internal/codemirror/bull-codemirror.ts \
   --bundle \
   --minify \
   --outfile=internal/codemirror/bull-codemirror.bundle.js

export NODE_PATH=$PWD/third_party/mermaid/node_modules
go tool esbuild \
   internal/mermaid/bull-mermaid.ts \
   --bundle \
   --minify \
   --outfile=internal/mermaid/bull-mermaid.bundle.js
