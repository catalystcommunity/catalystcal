package services

import (
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
)

// Register wires every service on the dispatcher, plus the identity check
// and the audit sink. The registry key of a service is its lower-case CSIL
// name without "Service"; the method is the PascalCase op. devAuth adds
// DevAuthService (the caller decides with Config.DevAuthAllowed).
func Register(d *csilrpc.Dispatcher, deps *Deps, devAuth bool) {
	d.SetIdentityChecker(deps.CheckIdentity)
	d.SetAuditRecorder(AuditRecorder{Store: deps.Store})

	a := &AuthService{deps}
	d.RegisterTypedPublic("auth", "Complete", csilrpc.Route(a.Complete, csil.DecodeCompleteRequest, csil.EncodeLoginResponse))
	d.RegisterTypedPublic("auth", "ResumeSession", csilrpc.Route(a.ResumeSession, csil.DecodeEmptyRequest, csil.EncodeLoginResponse))
	d.RegisterTypedPublic("auth", "EndSession", csilrpc.Route(a.EndSession, csil.DecodeEmptyRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("auth", "Me", csilrpc.Route(a.Me, csil.DecodeEmptyRequest, csil.EncodeMeResponse))
	d.RegisterTyped("auth", "AssumePrincipal", csilrpc.Route(a.AssumePrincipal, csil.DecodeAssumePrincipalRequest, csil.EncodeLoginResponse))
	d.RegisterTyped("auth", "ReleasePrincipal", csilrpc.Route(a.ReleasePrincipal, csil.DecodeEmptyRequest, csil.EncodeLoginResponse))
	d.RegisterTypedPublic("auth", "ExchangeActAsCredential", csilrpc.Route(a.ExchangeActAsCredential, csil.DecodeExchangeActAsCredentialRequest, csil.EncodeLoginResponse))
	d.RegisterTypedPublic("auth", "GetActAsScopeSet", csilrpc.Route(a.GetActAsScopeSet, csil.DecodeScopeSetRequest, csil.EncodeScopeSetResponse))

	if devAuth {
		da := &DevAuthService{deps}
		d.RegisterTypedPublic("devauth", "ListDevUsers", csilrpc.Route(da.ListDevUsers, csil.DecodeEmptyRequest, csil.EncodeDevUsersResponse))
		d.RegisterTypedPublic("devauth", "DevLogin", csilrpc.Route(da.DevLogin, csil.DecodeDevLoginRequest, csil.EncodeLoginResponse))
	}

	u := &UserService{deps}
	d.RegisterTyped("user", "GetProfile", csilrpc.Route(u.GetProfile, csil.DecodeEmptyRequest, csil.EncodeUser))
	d.RegisterTyped("user", "UpdateProfile", csilrpc.Route(u.UpdateProfile, csil.DecodeUpdateProfileRequest, csil.EncodeUser))
	d.RegisterTyped("user", "LookupPerson", csilrpc.Route(u.LookupPerson, csil.DecodeLookupPersonRequest, csil.EncodePersonSummary))

	c := &CalendarService{deps}
	d.RegisterTyped("calendar", "ListCalendars", csilrpc.Route(c.ListCalendars, csil.DecodeEmptyRequest, csil.EncodeCalendarList))
	d.RegisterTyped("calendar", "GetCalendar", csilrpc.Route(c.GetCalendar, csil.DecodeCalendarGetCalendarRequest, csil.EncodeCalendar))
	d.RegisterTyped("calendar", "CreateCalendar", csilrpc.Route(c.CreateCalendar, csil.DecodeCreateCalendarRequest, csil.EncodeCalendar))
	d.RegisterTyped("calendar", "UpdateCalendar", csilrpc.Route(c.UpdateCalendar, csil.DecodeUpdateCalendarRequest, csil.EncodeCalendar))
	d.RegisterTyped("calendar", "DeleteCalendar", csilrpc.Route(c.DeleteCalendar, csil.DecodeCalendarDeleteCalendarRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("calendar", "SubscribeCalendar", csilrpc.Route(c.SubscribeCalendar, csil.DecodeSubscribeCalendarRequest, csil.EncodeCalendar))
	d.RegisterTyped("calendar", "RefreshSubscription", csilrpc.Route(c.RefreshSubscription, csil.DecodeCalendarRefreshSubscriptionRequest, csil.EncodeCalendar))
	d.RegisterTyped("calendar", "SetFeed", csilrpc.Route(c.SetFeed, csil.DecodeSetFeedRequest, csil.EncodeCalendar))

	sh := &ShareService{deps}
	d.RegisterTyped("share", "ListShares", csilrpc.Route(sh.ListShares, csil.DecodeShareListSharesRequest, csil.EncodeShareList))
	d.RegisterTyped("share", "PutShare", csilrpc.Route(sh.PutShare, csil.DecodeShare, csil.EncodeShare))
	d.RegisterTyped("share", "DeleteShare", csilrpc.Route(sh.DeleteShare, csil.DecodeShareDeleteShareRequest, csil.EncodeEmptyResponse))

	dl := &DelegationService{deps}
	d.RegisterTyped("delegation", "ListGranted", csilrpc.Route(dl.ListGranted, csil.DecodeEmptyRequest, csil.EncodeDelegationList))
	d.RegisterTyped("delegation", "ListReceived", csilrpc.Route(dl.ListReceived, csil.DecodeEmptyRequest, csil.EncodeDelegationList))
	d.RegisterTyped("delegation", "CreateDelegation", csilrpc.Route(dl.CreateDelegation, csil.DecodeDelegation, csil.EncodeDelegation))
	d.RegisterTyped("delegation", "UpdateDelegation", csilrpc.Route(dl.UpdateDelegation, csil.DecodeDelegation, csil.EncodeDelegation))
	d.RegisterTyped("delegation", "RevokeDelegation", csilrpc.Route(dl.RevokeDelegation, csil.DecodeDelegationRevokeDelegationRequest, csil.EncodeEmptyResponse))

	e := &EventService{deps}
	d.RegisterTyped("event", "ListEvents", csilrpc.Route(e.ListEvents, csil.DecodeListEventsRequest, csil.EncodeOccurrenceList))
	d.RegisterTyped("event", "GetEvent", csilrpc.Route(e.GetEvent, csil.DecodeEventGetEventRequest, csil.EncodeEvent))
	d.RegisterTyped("event", "CreateEvent", csilrpc.Route(e.CreateEvent, csil.DecodeEvent, csil.EncodeEvent))
	d.RegisterTyped("event", "UpdateEvent", csilrpc.Route(e.UpdateEvent, csil.DecodeUpdateEventRequest, csil.EncodeEvent))
	d.RegisterTyped("event", "DeleteEvent", csilrpc.Route(e.DeleteEvent, csil.DecodeDeleteEventRequest, csil.EncodeEmptyResponse))
	d.RegisterTyped("event", "MoveEvent", csilrpc.Route(e.MoveEvent, csil.DecodeMoveEventRequest, csil.EncodeEvent))
	d.RegisterTyped("event", "Respond", csilrpc.Route(e.Respond, csil.DecodeRespondRequest, csil.EncodeEvent))
	d.RegisterTyped("event", "ListInvitations", csilrpc.Route(e.ListInvitations, csil.DecodeEmptyRequest, csil.EncodeEventList))
	d.RegisterTyped("event", "SearchEvents", csilrpc.Route(e.SearchEvents, csil.DecodeSearchEventsRequest, csil.EncodeEventList))

	fb := &FreeBusyService{deps}
	d.RegisterTyped("freebusy", "Query", csilrpc.Route(fb.Query, csil.DecodeFreeBusyRequest, csil.EncodeFreeBusyResponse))

	ic := &IcsService{deps}
	d.RegisterTyped("ics", "ImportIcs", csilrpc.Route(ic.ImportIcs, csil.DecodeImportIcsRequest, csil.EncodeImportResult))
	d.RegisterTyped("ics", "ExportCalendar", csilrpc.Route(ic.ExportCalendar, csil.DecodeExportCalendarRequest, csil.EncodeIcsDocument))
	d.RegisterTyped("ics", "ExportEvent", csilrpc.Route(ic.ExportEvent, csil.DecodeExportEventRequest, csil.EncodeIcsDocument))

	au := &AuditService{deps}
	d.RegisterTyped("audit", "ListAudit", csilrpc.Route(au.ListAudit, csil.DecodeAuditQuery, csil.EncodeAuditList))
}
