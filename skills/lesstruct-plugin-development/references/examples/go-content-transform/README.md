# Content Transform Plugin

A Lesstruct plugin that transforms content before it is saved by uppercasing the title.

## Prerequisites

- [TinyGo](https://tinygo.org/getting-started/install/) 0.28+

## Build

```bash
tinygo build -o content-transform.wasm -target=wasi main.go
```

## Install

Copy the compiled WASM file to the Lesstruct plugins directory:

```bash
cp content-transform.wasm <lesstruct-project>/plugins/
```

## Expected Behavior

When content is saved, the plugin intercepts the `before_save` hook and uppercases the `"title"` field value. For example:

- Input: `{"title":"my blog post", "content":"..."}`
- Output: `{"title":"MY BLOG POST", "content":"..."}`
