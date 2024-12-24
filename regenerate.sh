#!/bin/sh

export NODE_PATH=$PWD/third_party/codemirror/node_modules
go tool esbuild \
   third_party/codemirror/bull-codemirror.jsx \
   --bundle \
   --minify \
   --outfile=third_party/codemirror/bull-codemirror.bundle.js
