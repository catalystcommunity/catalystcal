// The carrier must speak the envelope the Go server decodes, map both error
// channels to ServiceError, and resume-and-retry exactly once on 401.
import { afterEach, describe, expect, it, vi } from "vitest";
import { RpcRequest, RpcResponse } from "./csil/rpc";
import { Status } from "./csil/conventions";
import { toServiceErrorCbor } from "@catalystcal/client";

vi.mock("~/stores/auth", () => ({ currentToken: () => "test-token" }));

import { ServiceError, cborTransport, methodToOp, setUnauthenticatedHandler, transportStatusToCode } from "./csilrpc";

let requests: RpcRequest[] = [];
let authHeaders: (string | undefined)[] = [];

function mockFetch(...replies: RpcResponse[]): void {
  let i = 0;
  globalThis.fetch = vi.fn(async (_url: unknown, init: { body?: BodyInit; headers?: Record<string, string> }) => {
    requests.push(RpcRequest.decode(new Uint8Array(init.body as Uint8Array)));
    authHeaders.push(init.headers?.Authorization);
    const reply = replies[Math.min(i++, replies.length - 1)];
    return new Response(reply.encode() as unknown as BodyInit, { status: 200 });
  }) as unknown as typeof fetch;
}

afterEach(() => {
  requests = [];
  authHeaders = [];
  setUnauthenticatedHandler(null);
  vi.restoreAllMocks();
});

describe("cborTransport", () => {
  it("sends the service and kebab op through, with the bearer, and returns the payload", async () => {
    const reqBytes = new Uint8Array([0xa1, 0x01, 0x02]);
    const respBytes = new Uint8Array([0x83, 0x01, 0x02, 0x03]);
    mockFetch(RpcResponse.ok("OccurrenceList", respBytes));

    const result = await cborTransport.call("EventService", "list-events", reqBytes);

    expect(requests[0].service).toBe("EventService");
    expect(requests[0].op).toBe("list-events");
    expect(requests[0].payload).toEqual(reqBytes);
    expect(result).toEqual(respBytes);
    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(call[0]).toBe("/api/csil/v1/rpc");
    expect(authHeaders[0]).toBe("Bearer test-token");
  });

  it("converts a PascalCase op to kebab case", () => {
    expect(methodToOp("ListDevUsers")).toBe("list-dev-users");
    expect(methodToOp("list-dev-users")).toBe("list-dev-users");
    expect(methodToOp("Me")).toBe("me");
  });

  it("throws ServiceError for the declared error arm", async () => {
    mockFetch(RpcResponse.ok("ServiceError", toServiceErrorCbor({ code: 403, message: "you cannot change this calendar" })));
    const err = await cborTransport.call("CalendarService", "update-calendar", new Uint8Array()).catch((e) => e);
    expect(err).toBeInstanceOf(ServiceError);
    expect(err).toMatchObject({ code: 403, message: "you cannot change this calendar" });
  });

  it("maps transport statuses to HTTP-shaped codes", async () => {
    mockFetch(RpcResponse.transportError(Status.UnknownServiceOrOp, "Unknown EventService operation: x"));
    const err = (await cborTransport.call("EventService", "x", new Uint8Array()).catch((e) => e)) as ServiceError;
    expect(err.code).toBe(404);
    expect(err.message).toBe("Unknown EventService operation: x");
    expect(transportStatusToCode(Status.Unauthenticated)).toBe(401);
    expect(transportStatusToCode(Status.Forbidden)).toBe(403);
    expect(transportStatusToCode(Status.MalformedEnvelope)).toBe(400);
    expect(transportStatusToCode(Status.Unavailable)).toBe(503);
    expect(transportStatusToCode(Status.DeadlineExceeded)).toBe(504);
    expect(transportStatusToCode(999)).toBe(500);
  });

  it("surfaces an HTTP failure without an envelope", async () => {
    globalThis.fetch = vi.fn(async () => new Response("bad gateway", { status: 502, statusText: "Bad Gateway" })) as unknown as typeof fetch;
    const err = (await cborTransport.call("EventService", "list-events", new Uint8Array()).catch((e) => e)) as ServiceError;
    expect(err.code).toBe(502);
  });

  it("resumes the session and retries once on 401", async () => {
    const ok = RpcResponse.ok("CalendarList", new Uint8Array([0xa0]));
    mockFetch(RpcResponse.transportError(Status.Unauthenticated, "expired"), ok);
    const resume = vi.fn(async () => true);
    setUnauthenticatedHandler(resume);
    const out = await cborTransport.call("CalendarService", "list-calendars", new Uint8Array());
    expect(out).toEqual(new Uint8Array([0xa0]));
    expect(resume).toHaveBeenCalledTimes(1);
    expect(requests.length).toBe(2);
  });

  it("retries only once, and not when the resume fails", async () => {
    const unauth = RpcResponse.transportError(Status.Unauthenticated, "expired");
    mockFetch(unauth, unauth, unauth);
    const resume = vi.fn(async () => true);
    setUnauthenticatedHandler(resume);
    const err = (await cborTransport.call("CalendarService", "list-calendars", new Uint8Array()).catch((e) => e)) as ServiceError;
    expect(err.code).toBe(401);
    expect(resume).toHaveBeenCalledTimes(1);
    expect(requests.length).toBe(2);

    requests = [];
    const failing = vi.fn(async () => false);
    setUnauthenticatedHandler(failing);
    await cborTransport.call("CalendarService", "list-calendars", new Uint8Array()).catch(() => undefined);
    expect(requests.length).toBe(1);
  });

  it("does not retry the session ops themselves", async () => {
    mockFetch(RpcResponse.transportError(Status.Unauthenticated, "no cookie"));
    const resume = vi.fn(async () => true);
    setUnauthenticatedHandler(resume);
    await cborTransport.call("AuthService", "resume-session", new Uint8Array()).catch(() => undefined);
    expect(resume).not.toHaveBeenCalled();
    expect(requests.length).toBe(1);
  });

  it("does not retry other errors", async () => {
    mockFetch(RpcResponse.ok("ServiceError", toServiceErrorCbor({ code: 404, message: "not found" })));
    const resume = vi.fn(async () => true);
    setUnauthenticatedHandler(resume);
    await cborTransport.call("EventService", "get-event", new Uint8Array()).catch(() => undefined);
    expect(resume).not.toHaveBeenCalled();
  });
});
