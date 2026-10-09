// Package kernel is the public contract of Lesstruct v1 ("everything is a module").
//
// Kernel owns the Module lifecycle, the Principal identity shape, the module
// registry, and the database driver/dialect contracts. It imports
// nothing from Lesstruct modules or internals, so dependencies always point one
// way: modules import kernel, the composition root imports everything.
//
// Versioning: the kernel is an independent Go module tagged kernel/vX.Y.Z.
// Semver rules, stability classes, and the deprecation flow live in
// docs/kernel-versioning.md. The one rule to internalize: adding a method to
// an exported interface is a breaking change — prefer new narrow interfaces
// over widening existing ones.
//
// Terminology: "modules" are compile-time Go modules (Tier 1). "Plugins" are the
// sandboxed WASM extensions (Tier 2) and live outside this package.
package kernel
