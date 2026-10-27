/**
 * One instance of each generated async client, all on the one CSIL-RPC
 * carrier. Every API call in the app goes through these, so a test can mock
 * this module.
 */
import {
  AuditAsyncClient,
  AuthAsyncClient,
  CalendarAsyncClient,
  DelegationAsyncClient,
  DevAuthAsyncClient,
  EventAsyncClient,
  FreeBusyAsyncClient,
  IcsAsyncClient,
  ShareAsyncClient,
  UserAsyncClient,
} from "@catalystcal/client";
import { cborTransport } from "~/transport/csilrpc";

export const authClient = new AuthAsyncClient(cborTransport);
export const devAuthClient = new DevAuthAsyncClient(cborTransport);
export const userClient = new UserAsyncClient(cborTransport);
export const calendarClient = new CalendarAsyncClient(cborTransport);
export const shareClient = new ShareAsyncClient(cborTransport);
export const delegationClient = new DelegationAsyncClient(cborTransport);
export const eventClient = new EventAsyncClient(cborTransport);
export const freeBusyClient = new FreeBusyAsyncClient(cborTransport);
export const icsClient = new IcsAsyncClient(cborTransport);
export const auditClient = new AuditAsyncClient(cborTransport);
