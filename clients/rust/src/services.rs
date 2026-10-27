//! Generated service traits from CSIL specification

use super::types::*;

/// AuthService service trait
pub trait AuthService {
    type Context;
    /// complete (request/response).
    fn complete(
        &self,
        ctx: &Self::Context,
        input: CompleteRequest,
    ) -> Result<LoginResponse, ServiceError>;
    /// resume-session (request/response).
    fn resume_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    /// end-session (request/response).
    fn end_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    /// me (request/response).
    fn me(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<MeResponse, ServiceError>;
    /// assume-principal (request/response).
    fn assume_principal(
        &self,
        ctx: &Self::Context,
        input: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ServiceError>;
    /// release-principal (request/response).
    fn release_principal(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    /// exchange-act-as-credential (request/response).
    fn exchange_act_as_credential(
        &self,
        ctx: &Self::Context,
        input: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ServiceError>;
    /// get-act-as-scope-set (request/response).
    fn get_act_as_scope_set(
        &self,
        ctx: &Self::Context,
        input: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ServiceError>;
}

/// DevAuthService service trait
pub trait DevAuthService {
    type Context;
    /// list-dev-users (request/response).
    fn list_dev_users(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DevUsersResponse, ServiceError>;
    /// dev-login (request/response).
    fn dev_login(
        &self,
        ctx: &Self::Context,
        input: DevLoginRequest,
    ) -> Result<LoginResponse, ServiceError>;
}

/// UserService service trait
pub trait UserService {
    type Context;
    /// get-profile (request/response).
    fn get_profile(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<User, ServiceError>;
    /// update-profile (request/response).
    fn update_profile(
        &self,
        ctx: &Self::Context,
        input: UpdateProfileRequest,
    ) -> Result<User, ServiceError>;
    /// lookup-person (request/response).
    fn lookup_person(
        &self,
        ctx: &Self::Context,
        input: LookupPersonRequest,
    ) -> Result<PersonSummary, ServiceError>;
}

/// CalendarService service trait
pub trait CalendarService {
    type Context;
    /// list-calendars (request/response).
    fn list_calendars(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<CalendarList, ServiceError>;
    /// get-calendar (request/response).
    fn get_calendar(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<Calendar, ServiceError>;
    /// create-calendar (request/response).
    fn create_calendar(
        &self,
        ctx: &Self::Context,
        input: CreateCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    /// update-calendar (request/response).
    fn update_calendar(
        &self,
        ctx: &Self::Context,
        input: UpdateCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    /// delete-calendar (request/response).
    fn delete_calendar(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<EmptyResponse, ServiceError>;
    /// subscribe-calendar (request/response).
    fn subscribe_calendar(
        &self,
        ctx: &Self::Context,
        input: SubscribeCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    /// refresh-subscription (request/response).
    fn refresh_subscription(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<Calendar, ServiceError>;
    /// set-feed (request/response).
    fn set_feed(
        &self,
        ctx: &Self::Context,
        input: SetFeedRequest,
    ) -> Result<Calendar, ServiceError>;
}

/// ShareService service trait
pub trait ShareService {
    type Context;
    /// list-shares (request/response).
    fn list_shares(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<ShareList, ServiceError>;
    /// put-share (request/response).
    fn put_share(&self, ctx: &Self::Context, input: Share) -> Result<Share, ServiceError>;
    /// delete-share (request/response).
    fn delete_share(
        &self,
        ctx: &Self::Context,
        input: ShareID,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// DelegationService service trait
pub trait DelegationService {
    type Context;
    /// list-granted (request/response).
    fn list_granted(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DelegationList, ServiceError>;
    /// list-received (request/response).
    fn list_received(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DelegationList, ServiceError>;
    /// create-delegation (request/response).
    fn create_delegation(
        &self,
        ctx: &Self::Context,
        input: Delegation,
    ) -> Result<Delegation, ServiceError>;
    /// update-delegation (request/response).
    fn update_delegation(
        &self,
        ctx: &Self::Context,
        input: Delegation,
    ) -> Result<Delegation, ServiceError>;
    /// revoke-delegation (request/response).
    fn revoke_delegation(
        &self,
        ctx: &Self::Context,
        input: DelegationID,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// EventService service trait
pub trait EventService {
    type Context;
    /// list-events (request/response).
    fn list_events(
        &self,
        ctx: &Self::Context,
        input: ListEventsRequest,
    ) -> Result<OccurrenceList, ServiceError>;
    /// get-event (request/response).
    fn get_event(&self, ctx: &Self::Context, input: EventID) -> Result<Event, ServiceError>;
    /// create-event (request/response).
    fn create_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    /// update-event (request/response).
    fn update_event(
        &self,
        ctx: &Self::Context,
        input: UpdateEventRequest,
    ) -> Result<Event, ServiceError>;
    /// delete-event (request/response).
    fn delete_event(
        &self,
        ctx: &Self::Context,
        input: DeleteEventRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    /// move-event (request/response).
    fn move_event(
        &self,
        ctx: &Self::Context,
        input: MoveEventRequest,
    ) -> Result<Event, ServiceError>;
    /// respond (request/response).
    fn respond(&self, ctx: &Self::Context, input: RespondRequest) -> Result<Event, ServiceError>;
    /// list-invitations (request/response).
    fn list_invitations(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EventList, ServiceError>;
    /// search-events (request/response).
    fn search_events(
        &self,
        ctx: &Self::Context,
        input: SearchEventsRequest,
    ) -> Result<EventList, ServiceError>;
}

/// FreeBusyService service trait
pub trait FreeBusyService {
    type Context;
    /// query (request/response).
    fn query(
        &self,
        ctx: &Self::Context,
        input: FreeBusyRequest,
    ) -> Result<FreeBusyResponse, ServiceError>;
}

/// IcsService service trait
pub trait IcsService {
    type Context;
    /// import-ics (request/response).
    fn import_ics(
        &self,
        ctx: &Self::Context,
        input: ImportIcsRequest,
    ) -> Result<ImportResult, ServiceError>;
    /// export-calendar (request/response).
    fn export_calendar(
        &self,
        ctx: &Self::Context,
        input: ExportCalendarRequest,
    ) -> Result<IcsDocument, ServiceError>;
    /// export-event (request/response).
    fn export_event(
        &self,
        ctx: &Self::Context,
        input: ExportEventRequest,
    ) -> Result<IcsDocument, ServiceError>;
}

/// AuditService service trait
pub trait AuditService {
    type Context;
    /// list-audit (request/response).
    fn list_audit(&self, ctx: &Self::Context, input: AuditQuery)
        -> Result<AuditList, ServiceError>;
}
