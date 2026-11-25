//! Generated service traits from CSIL specification

use super::types::*;

/// AuthService service trait
pub trait AuthService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// complete (request/response).
    fn complete(
        &self,
        ctx: &Self::Context,
        input: CompleteRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// resume-session (request/response).
    fn resume_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// end-session (request/response).
    fn end_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// me (request/response).
    fn me(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<MeResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// assume-principal (request/response).
    fn assume_principal(
        &self,
        ctx: &Self::Context,
        input: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// release-principal (request/response).
    fn release_principal(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// exchange-act-as-credential (request/response).
    fn exchange_act_as_credential(
        &self,
        ctx: &Self::Context,
        input: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-act-as-scope-set (request/response).
    fn get_act_as_scope_set(
        &self,
        ctx: &Self::Context,
        input: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-application-identity (request/response).
    fn get_application_identity(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<ApplicationIdentityResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// exchange-application-presentation (request/response).
    fn exchange_application_presentation(
        &self,
        ctx: &Self::Context,
        input: ExchangeApplicationPresentationRequest,
    ) -> Result<LoginResponse, ServiceError>;
}

/// DevAuthService service trait
pub trait DevAuthService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-dev-users (request/response).
    fn list_dev_users(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DevUsersResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// get-profile (request/response).
    fn get_profile(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<User, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-profile (request/response).
    fn update_profile(
        &self,
        ctx: &Self::Context,
        input: UpdateProfileRequest,
    ) -> Result<User, ServiceError>;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// list-calendars (request/response).
    fn list_calendars(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<CalendarList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-calendar (request/response).
    fn get_calendar(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<Calendar, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-calendar (request/response).
    fn create_calendar(
        &self,
        ctx: &Self::Context,
        input: CreateCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-calendar (request/response).
    fn update_calendar(
        &self,
        ctx: &Self::Context,
        input: UpdateCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-calendar (request/response).
    fn delete_calendar(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// subscribe-calendar (request/response).
    fn subscribe_calendar(
        &self,
        ctx: &Self::Context,
        input: SubscribeCalendarRequest,
    ) -> Result<Calendar, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// refresh-subscription (request/response).
    fn refresh_subscription(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<Calendar, ServiceError>;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// list-shares (request/response).
    fn list_shares(
        &self,
        ctx: &Self::Context,
        input: CalendarID,
    ) -> Result<ShareList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// put-share (request/response).
    fn put_share(&self, ctx: &Self::Context, input: Share) -> Result<Share, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-share (request/response).
    fn delete_share(
        &self,
        ctx: &Self::Context,
        input: ShareID,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// GroupService service trait
pub trait GroupService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-groups (request/response).
    fn list_groups(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<GroupList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-group (request/response).
    fn get_group(&self, ctx: &Self::Context, input: GroupID) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-group (request/response).
    fn create_group(
        &self,
        ctx: &Self::Context,
        input: CreateGroupRequest,
    ) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-group (request/response).
    fn update_group(
        &self,
        ctx: &Self::Context,
        input: UpdateGroupRequest,
    ) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// set-members (request/response).
    fn set_members(
        &self,
        ctx: &Self::Context,
        input: SetGroupMembersRequest,
    ) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-group (request/response).
    fn delete_group(
        &self,
        ctx: &Self::Context,
        input: GroupID,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// DelegationService service trait
pub trait DelegationService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-granted (request/response).
    fn list_granted(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DelegationList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-received (request/response).
    fn list_received(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DelegationList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-delegation (request/response).
    fn create_delegation(
        &self,
        ctx: &Self::Context,
        input: Delegation,
    ) -> Result<Delegation, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-delegation (request/response).
    fn update_delegation(
        &self,
        ctx: &Self::Context,
        input: Delegation,
    ) -> Result<Delegation, ServiceError>;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// list-events (request/response).
    fn list_events(
        &self,
        ctx: &Self::Context,
        input: ListEventsRequest,
    ) -> Result<OccurrenceList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-event (request/response).
    fn get_event(&self, ctx: &Self::Context, input: EventID) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-occurrence (request/response).
    fn get_occurrence(
        &self,
        ctx: &Self::Context,
        input: GetOccurrenceRequest,
    ) -> Result<Occurrence, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-event (request/response).
    fn create_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-event (request/response).
    fn update_event(
        &self,
        ctx: &Self::Context,
        input: UpdateEventRequest,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-event (request/response).
    fn delete_event(
        &self,
        ctx: &Self::Context,
        input: DeleteEventRequest,
    ) -> Result<DeleteEventResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// move-event (request/response).
    fn move_event(
        &self,
        ctx: &Self::Context,
        input: MoveEventRequest,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// respond (request/response).
    fn respond(&self, ctx: &Self::Context, input: RespondRequest) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-invitations (request/response).
    fn list_invitations(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EventList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// search-events (request/response).
    fn search_events(
        &self,
        ctx: &Self::Context,
        input: SearchEventsRequest,
    ) -> Result<EventList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-trash (request/response).
    fn list_trash(
        &self,
        ctx: &Self::Context,
        input: ListTrashRequest,
    ) -> Result<TrashList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// restore-event (request/response).
    fn restore_event(
        &self,
        ctx: &Self::Context,
        input: RestoreEventRequest,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// import-event (request/response).
    fn import_event(
        &self,
        ctx: &Self::Context,
        input: ImportEventRequest,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-event-once (request/response).
    fn create_event_once(
        &self,
        ctx: &Self::Context,
        input: CreateEventOnceRequest,
    ) -> Result<CreateEventReceipt, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-create-receipt (request/response).
    fn get_create_receipt(
        &self,
        ctx: &Self::Context,
        input: CreateReceiptRequest,
    ) -> Result<CreateEventReceipt, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// split-series-once (request/response).
    fn split_series_once(
        &self,
        ctx: &Self::Context,
        input: SplitSeriesOnceRequest,
    ) -> Result<SplitSeriesReceipt, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-split-receipt (request/response).
    fn get_split_receipt(
        &self,
        ctx: &Self::Context,
        input: SplitReceiptRequest,
    ) -> Result<SplitSeriesReceipt, ServiceError>;
}

/// FreeBusyService service trait
pub trait FreeBusyService {
    type Context;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// import-ics (request/response).
    fn import_ics(
        &self,
        ctx: &Self::Context,
        input: ImportIcsRequest,
    ) -> Result<ImportResult, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// export-calendar (request/response).
    fn export_calendar(
        &self,
        ctx: &Self::Context,
        input: ExportCalendarRequest,
    ) -> Result<IcsDocument, ServiceError>;
    #[allow(clippy::result_large_err)]
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
    #[allow(clippy::result_large_err)]
    /// list-audit (request/response).
    fn list_audit(
        &self,
        ctx: &Self::Context,
        input_value: AuditQuery,
    ) -> Result<AuditList, ServiceError>;
}
