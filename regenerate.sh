#!/bin/sh

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

export NODE_PATH=$PWD/third_party/codemirror/node_modules
go tool esbuild \
   internal/codemirror/bull-codemirror.ts \
   --bundle \
   --minify \
   --outfile=internal/codemirror/bull-codemirror.bundle.js
