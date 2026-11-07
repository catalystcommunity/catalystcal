//! Generated transport-agnostic service clients from CSIL specification

use super::codec::*;
use super::types::*;

/// Error from a generated client call: a structured error the service returned,
/// or a transport-level failure. The caller-supplied `Transport` decides how an
/// error response maps onto `Service`.
#[derive(Debug, Clone)]
pub enum ClientError {
    Service { code: i64, message: String },
    Transport(String),
}

impl std::fmt::Display for ClientError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ClientError::Service { code, message } => write!(f, "service error {code}: {message}"),
            ClientError::Transport(msg) => write!(f, "transport error: {msg}"),
        }
    }
}

impl std::error::Error for ClientError {}

/// The caller-supplied byte carrier: it performs the call named by `(service, op)`
/// with the already-encoded request bytes and returns the response bytes, or an
/// error. The generated client owns (de)serialization via the codec; the carrier
/// only moves bytes, so it can be HTTP, a queue, or an in-process loop.
pub trait Transport {
    fn call(&self, service: &str, op: &str, req: &[u8]) -> Result<Vec<u8>, ClientError>;
}

/// Typed client for the AuthService service.
pub struct AuthClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> AuthClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// complete (request/response).
    pub fn complete(&self, req: CompleteRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp =
            self.transport
                .call("AuthService", "complete", &encode_complete_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// resume-session (request/response).
    pub fn resume_session(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp =
            self.transport
                .call("AuthService", "resume-session", &encode_empty_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// end-session (request/response).
    pub fn end_session(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp =
            self.transport
                .call("AuthService", "end-session", &encode_empty_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// me (request/response).
    pub fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "me", &encode_empty_request(&req))?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// assume-principal (request/response).
    pub fn assume_principal(
        &self,
        req: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "assume-principal",
            &encode_assume_principal_request(&req),
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// release-principal (request/response).
    pub fn release_principal(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "release-principal",
            &encode_empty_request(&req),
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// exchange-act-as-credential (request/response).
    pub fn exchange_act_as_credential(
        &self,
        req: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "exchange-act-as-credential",
            &encode_exchange_act_as_credential_request(&req),
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-act-as-scope-set (request/response).
    pub fn get_act_as_scope_set(
        &self,
        req: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "get-act-as-scope-set",
            &encode_scope_set_request(&req),
        )?;
        decode_scope_set_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-application-identity (request/response).
    pub fn get_application_identity(
        &self,
        req: EmptyRequest,
    ) -> Result<ApplicationIdentityResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "get-application-identity",
            &encode_empty_request(&req),
        )?;
        decode_application_identity_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// exchange-application-presentation (request/response).
    pub fn exchange_application_presentation(
        &self,
        req: ExchangeApplicationPresentationRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call(
            "AuthService",
            "exchange-application-presentation",
            &encode_exchange_application_presentation_request(&req),
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DevAuthService service.
pub struct DevAuthClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> DevAuthClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-dev-users (request/response).
    pub fn list_dev_users(&self, req: EmptyRequest) -> Result<DevUsersResponse, ClientError> {
        let csil_resp = self.transport.call(
            "DevAuthService",
            "list-dev-users",
            &encode_empty_request(&req),
        )?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// dev-login (request/response).
    pub fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call(
            "DevAuthService",
            "dev-login",
            &encode_dev_login_request(&req),
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the UserService service.
pub struct UserClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> UserClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// get-profile (request/response).
    pub fn get_profile(&self, req: EmptyRequest) -> Result<User, ClientError> {
        let csil_resp =
            self.transport
                .call("UserService", "get-profile", &encode_empty_request(&req))?;
        decode_user(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-profile (request/response).
    pub fn update_profile(&self, req: UpdateProfileRequest) -> Result<User, ClientError> {
        let csil_resp = self.transport.call(
            "UserService",
            "update-profile",
            &encode_update_profile_request(&req),
        )?;
        decode_user(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// lookup-person (request/response).
    pub fn lookup_person(&self, req: LookupPersonRequest) -> Result<PersonSummary, ClientError> {
        let csil_resp = self.transport.call(
            "UserService",
            "lookup-person",
            &encode_lookup_person_request(&req),
        )?;
        decode_person_summary(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the CalendarService service.
pub struct CalendarClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> CalendarClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-calendars (request/response).
    pub fn list_calendars(&self, req: EmptyRequest) -> Result<CalendarList, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "list-calendars",
            &encode_empty_request(&req),
        )?;
        decode_calendar_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-calendar (request/response).
    pub fn get_calendar(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "get-calendar",
            &encode_calendar_get_calendar_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-calendar (request/response).
    pub fn create_calendar(&self, req: CreateCalendarRequest) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "create-calendar",
            &encode_create_calendar_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-calendar (request/response).
    pub fn update_calendar(&self, req: UpdateCalendarRequest) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "update-calendar",
            &encode_update_calendar_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-calendar (request/response).
    pub fn delete_calendar(&self, req: CalendarID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "delete-calendar",
            &encode_calendar_delete_calendar_request(&req),
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// subscribe-calendar (request/response).
    pub fn subscribe_calendar(
        &self,
        req: SubscribeCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "subscribe-calendar",
            &encode_subscribe_calendar_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// refresh-subscription (request/response).
    pub fn refresh_subscription(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "refresh-subscription",
            &encode_calendar_refresh_subscription_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-feed (request/response).
    pub fn set_feed(&self, req: SetFeedRequest) -> Result<Calendar, ClientError> {
        let csil_resp = self.transport.call(
            "CalendarService",
            "set-feed",
            &encode_set_feed_request(&req),
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ShareService service.
pub struct ShareClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> ShareClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-shares (request/response).
    pub fn list_shares(&self, req: CalendarID) -> Result<ShareList, ClientError> {
        let csil_resp = self.transport.call(
            "ShareService",
            "list-shares",
            &encode_share_list_shares_request(&req),
        )?;
        decode_share_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-share (request/response).
    pub fn put_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_resp = self
            .transport
            .call("ShareService", "put-share", &encode_share(&req))?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-share (request/response).
    pub fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call(
            "ShareService",
            "delete-share",
            &encode_share_delete_share_request(&req),
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the GroupService service.
pub struct GroupClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> GroupClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-groups (request/response).
    pub fn list_groups(&self, req: EmptyRequest) -> Result<GroupList, ClientError> {
        let csil_resp =
            self.transport
                .call("GroupService", "list-groups", &encode_empty_request(&req))?;
        decode_group_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-group (request/response).
    pub fn get_group(&self, req: GroupID) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call(
            "GroupService",
            "get-group",
            &encode_group_get_group_request(&req),
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-group (request/response).
    pub fn create_group(&self, req: CreateGroupRequest) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call(
            "GroupService",
            "create-group",
            &encode_create_group_request(&req),
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-group (request/response).
    pub fn update_group(&self, req: UpdateGroupRequest) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call(
            "GroupService",
            "update-group",
            &encode_update_group_request(&req),
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-members (request/response).
    pub fn set_members(&self, req: SetGroupMembersRequest) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call(
            "GroupService",
            "set-members",
            &encode_set_group_members_request(&req),
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-group (request/response).
    pub fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call(
            "GroupService",
            "delete-group",
            &encode_group_delete_group_request(&req),
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DelegationService service.
pub struct DelegationClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> DelegationClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-granted (request/response).
    pub fn list_granted(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_resp = self.transport.call(
            "DelegationService",
            "list-granted",
            &encode_empty_request(&req),
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-received (request/response).
    pub fn list_received(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_resp = self.transport.call(
            "DelegationService",
            "list-received",
            &encode_empty_request(&req),
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-delegation (request/response).
    pub fn create_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_resp = self.transport.call(
            "DelegationService",
            "create-delegation",
            &encode_delegation(&req),
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-delegation (request/response).
    pub fn update_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_resp = self.transport.call(
            "DelegationService",
            "update-delegation",
            &encode_delegation(&req),
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// revoke-delegation (request/response).
    pub fn revoke_delegation(&self, req: DelegationID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call(
            "DelegationService",
            "revoke-delegation",
            &encode_delegation_revoke_delegation_request(&req),
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the EventService service.
pub struct EventClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> EventClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-events (request/response).
    pub fn list_events(&self, req: ListEventsRequest) -> Result<OccurrenceList, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "list-events",
            &encode_list_events_request(&req),
        )?;
        decode_occurrence_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-event (request/response).
    pub fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "get-event",
            &encode_event_get_event_request(&req),
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-occurrence (request/response).
    pub fn get_occurrence(&self, req: GetOccurrenceRequest) -> Result<Occurrence, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "get-occurrence",
            &encode_get_occurrence_request(&req),
        )?;
        decode_occurrence(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-event (request/response).
    pub fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call("EventService", "create-event", &encode_event(&req))?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-event (request/response).
    pub fn update_event(&self, req: UpdateEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "update-event",
            &encode_update_event_request(&req),
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event (request/response).
    pub fn delete_event(
        &self,
        req: DeleteEventRequest,
    ) -> Result<DeleteEventResponse, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "delete-event",
            &encode_delete_event_request(&req),
        )?;
        decode_delete_event_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// move-event (request/response).
    pub fn move_event(&self, req: MoveEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "move-event",
            &encode_move_event_request(&req),
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// respond (request/response).
    pub fn respond(&self, req: RespondRequest) -> Result<Event, ClientError> {
        let csil_resp =
            self.transport
                .call("EventService", "respond", &encode_respond_request(&req))?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-invitations (request/response).
    pub fn list_invitations(&self, req: EmptyRequest) -> Result<EventList, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "list-invitations",
            &encode_empty_request(&req),
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// search-events (request/response).
    pub fn search_events(&self, req: SearchEventsRequest) -> Result<EventList, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "search-events",
            &encode_search_events_request(&req),
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-trash (request/response).
    pub fn list_trash(&self, req: ListTrashRequest) -> Result<TrashList, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "list-trash",
            &encode_list_trash_request(&req),
        )?;
        decode_trash_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// restore-event (request/response).
    pub fn restore_event(&self, req: RestoreEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "restore-event",
            &encode_restore_event_request(&req),
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// import-event (request/response).
    pub fn import_event(&self, req: ImportEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "import-event",
            &encode_import_event_request(&req),
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-event-once (request/response).
    pub fn create_event_once(
        &self,
        req: CreateEventOnceRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "create-event-once",
            &encode_create_event_once_request(&req),
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-create-receipt (request/response).
    pub fn get_create_receipt(
        &self,
        req: CreateReceiptRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "get-create-receipt",
            &encode_create_receipt_request(&req),
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// split-series-once (request/response).
    pub fn split_series_once(
        &self,
        req: SplitSeriesOnceRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "split-series-once",
            &encode_split_series_once_request(&req),
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-split-receipt (request/response).
    pub fn get_split_receipt(
        &self,
        req: SplitReceiptRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_resp = self.transport.call(
            "EventService",
            "get-split-receipt",
            &encode_split_receipt_request(&req),
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the FreeBusyService service.
pub struct FreeBusyClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> FreeBusyClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// query (request/response).
    pub fn query(&self, req: FreeBusyRequest) -> Result<FreeBusyResponse, ClientError> {
        let csil_resp =
            self.transport
                .call("FreeBusyService", "query", &encode_free_busy_request(&req))?;
        decode_free_busy_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the IcsService service.
pub struct IcsClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> IcsClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// import-ics (request/response).
    pub fn import_ics(&self, req: ImportIcsRequest) -> Result<ImportResult, ClientError> {
        let csil_resp =
            self.transport
                .call("IcsService", "import-ics", &encode_import_ics_request(&req))?;
        decode_import_result(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// export-calendar (request/response).
    pub fn export_calendar(&self, req: ExportCalendarRequest) -> Result<IcsDocument, ClientError> {
        let csil_resp = self.transport.call(
            "IcsService",
            "export-calendar",
            &encode_export_calendar_request(&req),
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// export-event (request/response).
    pub fn export_event(&self, req: ExportEventRequest) -> Result<IcsDocument, ClientError> {
        let csil_resp = self.transport.call(
            "IcsService",
            "export-event",
            &encode_export_event_request(&req),
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the AuditService service.
pub struct AuditClient<T: Transport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: Transport> AuditClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-audit (request/response).
    pub fn list_audit(&self, req: AuditQuery) -> Result<AuditList, ClientError> {
        let csil_resp =
            self.transport
                .call("AuditService", "list-audit", &encode_audit_query(&req))?;
        decode_audit_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}
