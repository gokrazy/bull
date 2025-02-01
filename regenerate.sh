#!/bin/sh

export NODE_PATH=$PWD/third_party/codemirror/node_modules
go tool esbuild \
   internal/codemirror/bull-codemirror.ts \
   --bundle \
   --minify \
   --outfile=internal/codemirror/bull-codemirror.bundle.js
