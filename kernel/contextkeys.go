package kernel

// principalKey is the sole context key carrying a Principal.
//
// The type is unexported so no other package can collide with or forge the key;
// use ContextWithPrincipal and PrincipalFromContext to write and read it.
type principalKey struct{}
