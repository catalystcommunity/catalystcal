// Package transport is a VENDORED copy of the canonical CSIL-RPC reference
// transport, github.com/catalystcommunity/csilgen/transports/go.
//
// It is copied here verbatim (cbor.go, conventions.go, carrier.go, rpc.go and
// the conformance/ vectors) rather than pulled as a module dependency so the
// build stays hermetic while the upstream module stabilizes — the same call
// linkkeys made when it vendored the Rust sibling. When the module is published
// and CI can fetch it, replace this directory with a normal require + import and
// drop the local conformance test.
//
// Do not hand-edit the copied files; re-copy them from upstream so the
// conformance vectors keep passing on both ends of the wire.
package transport
