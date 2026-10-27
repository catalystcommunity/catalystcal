/**
 * VENDORED copy of the canonical CSIL-RPC reference TypeScript transport
 * (`csilgen-transport`, from csilgen/transports/typescript).
 *
 * Copied here verbatim (cbor.ts, conventions.ts, carrier.ts, rpc.ts) — with the
 * sole edit of stripping `.ts` import extensions to suit Bundler resolution —
 * rather than depending on the unpublished npm package, so the build stays
 * self-contained while upstream stabilizes. This mirrors what linkkeys did when
 * it vendored the Rust sibling. The codec is byte-compatible with the Go and
 * Rust references (verified by the shared conformance vectors).
 *
 * Do not hand-edit the copied files; re-copy from upstream so the wire stays in
 * lockstep with the server.
 */
export { RpcRequest, RpcResponse } from "./rpc";
export { Status, TransportError, statusName } from "./conventions";
export { encode as encodeCbor, decode as decodeCbor } from "./cbor";
