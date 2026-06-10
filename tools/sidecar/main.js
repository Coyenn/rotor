#!/usr/bin/env node

// The line-JSON protocol owns stdout. Plugins are arbitrary user code that
// console.log()s freely (Flamework does), so reserve the real stdout writer
// for protocol responses and reroute every other stdout write to stderr.
const protocolWrite = process.stdout.write.bind(process.stdout);
process.stdout.write = (chunk, encoding, callback) => process.stderr.write(chunk, encoding, callback);

const { serveStdio } = require("./index.js");

serveStdio({ output: { write: (text) => protocolWrite(text) } });
