# Validation Plugin

A Lesstruct plugin that validates content before it is saved by rejecting empty titles.

## Prerequisites

- [TinyGo](https://tinygo.org/getting-started/install/) 0.28+

## Build

```bash
tinygo build -o validation.wasm -target=wasi main.go
```

## Install

Copy the compiled WASM file to the Lesstruct plugins directory:

```bash
cp validation.wasm <lesstruct-project>/plugins/
```

## Expected Behavior

When content is saved, the plugin intercepts the `before_save` hook and checks if the `"title"` field is empty.

- If the title is empty, the plugin returns an error response:
  ```json
  {"error":"validation failed","message":"title must not be empty"}
  ```
- If the title is not empty, the original content passes through unchanged.
