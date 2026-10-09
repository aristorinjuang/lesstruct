// Package capability defines the Published Language of authorization: the
// capability names every module checks and the users module resolves. Names
// are stable strings so modules never hardcode them and no second vocabulary
// drifts elsewhere. Unknown names fail closed at the resolver.
package capability

import (
	"strings"
)

// Capability names a permission the kernel resolver decides.
type Capability string

const (
	// CapabilityAdmin is the reserved superuser grant.
	CapabilityAdmin Capability = "admin"
	// CapabilityMedia grants media upload/generation.
	CapabilityMedia Capability = "media"
	// CapabilityComments grants authenticated commenting.
	CapabilityComments Capability = "comments"
	// CapabilityPublishContent grants direct publishing.
	CapabilityPublishContent Capability = "content.publish"
	// CapabilityManageContent is the prefix for per-type manage grants of
	// the form "content.manage:<post-type>". Use ManageContent to build it.
	CapabilityManageContent Capability = "content.manage"
)

// ManageContent builds the per-type manage capability for a post-type slug.
func ManageContent(postType string) string {
	return string(CapabilityManageContent) + ":" + postType
}

// Split breaks a capability into its name and argument ("content.manage",
// "post" for "content.manage:post"). A bare name yields an empty argument.
func Split(capability string) (name Capability, arg string) {
	head, tail, _ := strings.Cut(capability, ":")
	return Capability(head), tail
}
