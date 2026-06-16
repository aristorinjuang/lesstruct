# Hello World Plugin

A minimal Lesstruct plugin that responds when loaded by the plugin system.

## Prerequisites

- [TinyGo](https://tinygo.org/getting-started/install/) 0.28+

## Build

```bash
tinygo build -o hello-world.wasm -target=wasi main.go
```

## Install

Copy the compiled WASM file to the Lesstruct plugins directory:

```bash
cp hello-world.wasm <lesstruct-project>/plugins/
```

## Expected Behavior

When Lesstruct starts, it loads the plugin and calls `hook_on_plugin_loaded`. The plugin returns a JSON response:

```json
{"status":"loaded","plugin":"hello-world","input_size":"..."}
```
