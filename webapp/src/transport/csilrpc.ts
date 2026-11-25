/**
 * CSIL-RPC carrier for the generated async clients (@catalystcal/client).
 *
 * The generated client owns (de)serialization. This carrier is the byte seam
 * it calls with the service, operation, and encoded request. Each call
 * wraps the encoded request in a CSIL-RPC envelope and POSTs it:
 *
 *   POST /api/csil/v1/rpc
 *   Content-Type: application/cbor
 *   body  = CsilRpcRequest { v, service, op, payload: 24(reqBytes) }
 *   reply = CsilRpcResponse { v, status, payload: 24(respBytes), ? variant, ? error }
 *
 * The generated client passes the full CSIL service name ("EventService")
 * and an already-kebab op ("list-events"). The server registry normalizes
 * the service name, so it goes through unchanged. The op goes through
 * methodToOp, which leaves a kebab op alone and converts a PascalCase one.
 *
 * Two failure channels, never mixed up:
 *   - a non-zero transport `status` (unauthenticated, unknown op, ...)
 *   - status 0 with `variant === "ServiceError"`: the declared error arm.
 * Generated clients decode the declared arm and throw ServiceErrorRefusal.
 */

import { type AsyncServiceTransport, ServiceErrorRefusal, fromServiceErrorCbor } from "@catalystcal/client";
import { currentToken } from "~/stores/auth";
import { RpcRequest, RpcResponse, Status, statusName } from "~/transport/csil";

export class ServiceError extends Error {
  constructor(
    message: string,
    public readonly code: number,
  ) {
    super(message);
    this.name = "ServiceError";
  }
}

const CONTENT_TYPE = "application/cbor";
export const RPC_ENDPOINT = "/api/csil/v1/rpc";

// lib/session registers resumeSession here. The transport cannot import it
// (that module imports the clients, which import this one). It resolves
// true when there is a usable bearer to retry with.
let onUnauthenticated: (() => Promise<boolean>) | null = null;
export function setUnauthenticatedHandler(fn: (() => Promise<boolean>) | null): void {
  onUnauthenticated = fn;
}

// Ops that must not trigger a resume-and-retry.
const NO_RETRY_OPS = new Set(["resume-session", "end-session", "complete", "dev-login"]);

export const cborTransport: AsyncServiceTransport = {
  async call(service, op, req) {
    const reply = await callReply(service, op, req);
    if (reply.variant === "ServiceError") {
      const { code, message } = fromServiceErrorCbor(reply.payload);
      throw new ServiceError(message, code);
    }
    return reply.payload;
  },
  callReply,
};

async function callReply(service: string, op: string, req: Uint8Array): Promise<RpcResponse> {
  const kebab = methodToOp(op);
  let reply: RpcResponse;
  try {
    reply = await send(service, kebab, req);
  } catch (e) {
    if (!(e instanceof ServiceError) || e.code !== 401 || !onUnauthenticated || NO_RETRY_OPS.has(kebab)) throw e;
    if (!(await onUnauthenticated())) throw e;
    return send(service, kebab, req);
  }
  if (reply.variant !== "ServiceError" || !onUnauthenticated || NO_RETRY_OPS.has(kebab)) return reply;
  if (fromServiceErrorCbor(reply.payload).code !== 401 || !(await onUnauthenticated())) return reply;
  return send(service, kebab, req);
}

async function send(service: string, op: string, req: Uint8Array): Promise<RpcResponse> {
  const headers: Record<string, string> = { "Content-Type": CONTENT_TYPE, Accept: CONTENT_TYPE };
  const tok = currentToken();
  if (tok) headers["Authorization"] = `Bearer ${tok}`;

  const envelope = new RpcRequest(service, op, req).encode();

  let res: Response;
  try {
    res = await fetch(RPC_ENDPOINT, { method: "POST", headers, body: new Uint8Array(envelope) });
  } catch (e) {
    throw new ServiceError("Cannot reach the catalystcal server. Check your connection and try again.", 0);
  }
  const buf = new Uint8Array(await res.arrayBuffer());

  // The carrier answers 200 with an envelope for every outcome. A non-200
  // without a decodable envelope is a proxy or network failure.
  let resp: RpcResponse;
  try {
    resp = RpcResponse.decode(buf);
  } catch {
    throw new ServiceError(res.ok ? "invalid response envelope" : `${res.status} ${res.statusText}`.trim(), res.ok ? 500 : res.status);
  }

  if (resp.status !== Status.Ok) {
    throw new ServiceError(resp.error ?? statusName(resp.status), transportStatusToCode(resp.status));
  }

  return resp;
}

/** "ListEvents" -> "list-events"; "list-events" stays as it is. */
export function methodToOp(method: string): string {
  return method.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

/** Map a CSIL-RPC transport status onto an HTTP-shaped code. */
export function transportStatusToCode(status: number): number {
  switch (status) {
    case Status.MalformedEnvelope:
      return 400;
    case Status.UnknownServiceOrOp:
      return 404;
    case Status.Unauthenticated:
      return 401;
    case Status.Forbidden:
      return 403;
    case Status.Unavailable:
      return 503;
    case Status.DeadlineExceeded:
      return 504;
    default:
      return 500;
  }
}

/** A message for the UI from any thrown value. */
export function errorMessage(e: unknown): string {
  if (e instanceof ServiceErrorRefusal) return e.value.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
