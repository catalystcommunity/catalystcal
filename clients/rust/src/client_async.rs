//! Generated transport-agnostic service clients from CSIL specification

#![allow(async_fn_in_trait)]

use super::client::ClientError;
use super::codec::*;
use super::types::*;

/// The caller-supplied byte carrier: it performs the call named by `(service, op)`
/// with the already-encoded request bytes and returns the response bytes, or an
/// error. The generated client owns (de)serialization via the codec; the carrier
/// only moves bytes, so it can be HTTP, a queue, or an in-process loop.
pub trait AsyncTransport {
    async fn call(&self, service: &str, op: &str, req: &[u8]) -> Result<Vec<u8>, ClientError>;
}

/// Typed client for the AuthService service.
pub struct AuthAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> AuthAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// complete (request/response).
    pub async fn complete(&self, req: CompleteRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "complete", &encode_complete_request(&req))
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// resume-session (request/response).
    pub async fn resume_session(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "resume-session", &encode_empty_request(&req))
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// end-session (request/response).
    pub async fn end_session(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "end-session", &encode_empty_request(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// me (request/response).
    pub async fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "me", &encode_empty_request(&req))
            .await?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// assume-principal (request/response).
    pub async fn assume_principal(
        &self,
        req: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "assume-principal",
                &encode_assume_principal_request(&req),
            )
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// release-principal (request/response).
    pub async fn release_principal(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "release-principal",
                &encode_empty_request(&req),
            )
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// exchange-act-as-credential (request/response).
    pub async fn exchange_act_as_credential(
        &self,
        req: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "exchange-act-as-credential",
                &encode_exchange_act_as_credential_request(&req),
            )
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-act-as-scope-set (request/response).
    pub async fn get_act_as_scope_set(
        &self,
        req: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "get-act-as-scope-set",
                &encode_scope_set_request(&req),
            )
            .await?;
        decode_scope_set_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DevAuthService service.
pub struct DevAuthAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> DevAuthAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-dev-users (request/response).
    pub async fn list_dev_users(&self, req: EmptyRequest) -> Result<DevUsersResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DevAuthService",
                "list-dev-users",
                &encode_empty_request(&req),
            )
            .await?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// dev-login (request/response).
    pub async fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DevAuthService",
                "dev-login",
                &encode_dev_login_request(&req),
            )
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the UserService service.
pub struct UserAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> UserAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// get-profile (request/response).
    pub async fn get_profile(&self, req: EmptyRequest) -> Result<User, ClientError> {
        let csil_resp = self
            .transport
            .call("UserService", "get-profile", &encode_empty_request(&req))
            .await?;
        decode_user(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-profile (request/response).
    pub async fn update_profile(&self, req: UpdateProfileRequest) -> Result<User, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "UserService",
                "update-profile",
                &encode_update_profile_request(&req),
            )
            .await?;
        decode_user(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// lookup-person (request/response).
    pub async fn lookup_person(
        &self,
        req: LookupPersonRequest,
    ) -> Result<PersonSummary, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "UserService",
                "lookup-person",
                &encode_lookup_person_request(&req),
            )
            .await?;
        decode_person_summary(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the CalendarService service.
pub struct CalendarAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> CalendarAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-calendars (request/response).
    pub async fn list_calendars(&self, req: EmptyRequest) -> Result<CalendarList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "list-calendars",
                &encode_empty_request(&req),
            )
            .await?;
        decode_calendar_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-calendar (request/response).
    pub async fn get_calendar(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "get-calendar",
                &encode_calendar_get_calendar_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-calendar (request/response).
    pub async fn create_calendar(
        &self,
        req: CreateCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "create-calendar",
                &encode_create_calendar_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-calendar (request/response).
    pub async fn update_calendar(
        &self,
        req: UpdateCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "update-calendar",
                &encode_update_calendar_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-calendar (request/response).
    pub async fn delete_calendar(&self, req: CalendarID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "delete-calendar",
                &encode_calendar_delete_calendar_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// subscribe-calendar (request/response).
    pub async fn subscribe_calendar(
        &self,
        req: SubscribeCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "subscribe-calendar",
                &encode_subscribe_calendar_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// refresh-subscription (request/response).
    pub async fn refresh_subscription(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "refresh-subscription",
                &encode_calendar_refresh_subscription_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-feed (request/response).
    pub async fn set_feed(&self, req: SetFeedRequest) -> Result<Calendar, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CalendarService",
                "set-feed",
                &encode_set_feed_request(&req),
            )
            .await?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ShareService service.
pub struct ShareAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> ShareAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-shares (request/response).
    pub async fn list_shares(&self, req: CalendarID) -> Result<ShareList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ShareService",
                "list-shares",
                &encode_share_list_shares_request(&req),
            )
            .await?;
        decode_share_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-share (request/response).
    pub async fn put_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_resp = self
            .transport
            .call("ShareService", "put-share", &encode_share(&req))
            .await?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-share (request/response).
    pub async fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ShareService",
                "delete-share",
                &encode_share_delete_share_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DelegationService service.
pub struct DelegationAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> DelegationAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-granted (request/response).
    pub async fn list_granted(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DelegationService",
                "list-granted",
                &encode_empty_request(&req),
            )
            .await?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-received (request/response).
    pub async fn list_received(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DelegationService",
                "list-received",
                &encode_empty_request(&req),
            )
            .await?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-delegation (request/response).
    pub async fn create_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DelegationService",
                "create-delegation",
                &encode_delegation(&req),
            )
            .await?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-delegation (request/response).
    pub async fn update_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DelegationService",
                "update-delegation",
                &encode_delegation(&req),
            )
            .await?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// revoke-delegation (request/response).
    pub async fn revoke_delegation(&self, req: DelegationID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DelegationService",
                "revoke-delegation",
                &encode_delegation_revoke_delegation_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the EventService service.
pub struct EventAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> EventAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-events (request/response).
    pub async fn list_events(&self, req: ListEventsRequest) -> Result<OccurrenceList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "list-events",
                &encode_list_events_request(&req),
            )
            .await?;
        decode_occurrence_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-event (request/response).
    pub async fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "get-event",
                &encode_event_get_event_request(&req),
            )
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-event (request/response).
    pub async fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call("EventService", "create-event", &encode_event(&req))
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-event (request/response).
    pub async fn update_event(&self, req: UpdateEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "update-event",
                &encode_update_event_request(&req),
            )
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event (request/response).
    pub async fn delete_event(
        &self,
        req: DeleteEventRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "delete-event",
                &encode_delete_event_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// move-event (request/response).
    pub async fn move_event(&self, req: MoveEventRequest) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "move-event",
                &encode_move_event_request(&req),
            )
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// respond (request/response).
    pub async fn respond(&self, req: RespondRequest) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call("EventService", "respond", &encode_respond_request(&req))
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-invitations (request/response).
    pub async fn list_invitations(&self, req: EmptyRequest) -> Result<EventList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "list-invitations",
                &encode_empty_request(&req),
            )
            .await?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// search-events (request/response).
    pub async fn search_events(&self, req: SearchEventsRequest) -> Result<EventList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "search-events",
                &encode_search_events_request(&req),
            )
            .await?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the FreeBusyService service.
pub struct FreeBusyAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> FreeBusyAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// query (request/response).
    pub async fn query(&self, req: FreeBusyRequest) -> Result<FreeBusyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("FreeBusyService", "query", &encode_free_busy_request(&req))
            .await?;
        decode_free_busy_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the IcsService service.
pub struct IcsAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> IcsAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// import-ics (request/response).
    pub async fn import_ics(&self, req: ImportIcsRequest) -> Result<ImportResult, ClientError> {
        let csil_resp = self
            .transport
            .call("IcsService", "import-ics", &encode_import_ics_request(&req))
            .await?;
        decode_import_result(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// export-calendar (request/response).
    pub async fn export_calendar(
        &self,
        req: ExportCalendarRequest,
    ) -> Result<IcsDocument, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "IcsService",
                "export-calendar",
                &encode_export_calendar_request(&req),
            )
            .await?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// export-event (request/response).
    pub async fn export_event(&self, req: ExportEventRequest) -> Result<IcsDocument, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "IcsService",
                "export-event",
                &encode_export_event_request(&req),
            )
            .await?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the AuditService service.
pub struct AuditAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> AuditAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-audit (request/response).
    pub async fn list_audit(&self, req: AuditQuery) -> Result<AuditList, ClientError> {
        let csil_resp = self
            .transport
            .call("AuditService", "list-audit", &encode_audit_query(&req))
            .await?;
        decode_audit_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}
