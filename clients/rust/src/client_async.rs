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

    /// Perform a call whose operation declares several reply arms and return
    /// the status-zero reply `variant` with its payload, so the client can
    /// return the success arm or the declared error arm.
    async fn call_reply(
        &self,
        service: &str,
        op: &str,
        req: &[u8],
    ) -> Result<super::client::WireReply, ClientError>;
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
        let csil_req = match encode_complete_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "complete", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "complete",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// resume-session (request/response).
    pub async fn resume_session(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "resume-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "resume-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// end-session (request/response).
    pub async fn end_session(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "end-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "end-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// me (request/response).
    pub async fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "me", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "me",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// assume-principal (request/response).
    pub async fn assume_principal(
        &self,
        req: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_assume_principal_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "assume-principal", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "assume-principal",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// release-principal (request/response).
    pub async fn release_principal(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "release-principal", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "release-principal",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-act-as-credential (request/response).
    pub async fn exchange_act_as_credential(
        &self,
        req: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_exchange_act_as_credential_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "exchange-act-as-credential", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "exchange-act-as-credential",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-act-as-scope-set (request/response).
    pub async fn get_act_as_scope_set(
        &self,
        req: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ClientError> {
        let csil_req = match encode_scope_set_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "get-act-as-scope-set", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "get-act-as-scope-set",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_scope_set_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-application-identity (request/response).
    pub async fn get_application_identity(
        &self,
        req: EmptyRequest,
    ) -> Result<ApplicationIdentityResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "get-application-identity", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "get-application-identity",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_application_identity_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-application-presentation (request/response).
    pub async fn exchange_application_presentation(
        &self,
        req: ExchangeApplicationPresentationRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_exchange_application_presentation_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply(
                "AuthService",
                "exchange-application-presentation",
                &csil_req,
            )
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "exchange-application-presentation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DevAuthService", "list-dev-users", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DevAuthService",
            "list-dev-users",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// dev-login (request/response).
    pub async fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_dev_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DevAuthService", "dev-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DevAuthService",
            "dev-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "get-profile", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "UserService",
            "get-profile",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_user(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-profile (request/response).
    pub async fn update_profile(&self, req: UpdateProfileRequest) -> Result<User, ClientError> {
        let csil_req = match encode_update_profile_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "update-profile", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "UserService",
            "update-profile",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_user(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// lookup-person (request/response).
    pub async fn lookup_person(
        &self,
        req: LookupPersonRequest,
    ) -> Result<PersonSummary, ClientError> {
        let csil_req = match encode_lookup_person_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "lookup-person", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "UserService",
            "lookup-person",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_person_summary(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "list-calendars", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "list-calendars",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-calendar (request/response).
    pub async fn get_calendar(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_req = match encode_calendar_get_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "get-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "get-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-calendar (request/response).
    pub async fn create_calendar(
        &self,
        req: CreateCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_req = match encode_create_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "create-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "create-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-calendar (request/response).
    pub async fn update_calendar(
        &self,
        req: UpdateCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_req = match encode_update_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "update-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "update-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-calendar (request/response).
    pub async fn delete_calendar(&self, req: CalendarID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_calendar_delete_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "delete-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "delete-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// subscribe-calendar (request/response).
    pub async fn subscribe_calendar(
        &self,
        req: SubscribeCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_req = match encode_subscribe_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "subscribe-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "subscribe-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// refresh-subscription (request/response).
    pub async fn refresh_subscription(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_req = match encode_calendar_refresh_subscription_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "refresh-subscription", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "refresh-subscription",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-feed (request/response).
    pub async fn set_feed(&self, req: SetFeedRequest) -> Result<Calendar, ClientError> {
        let csil_req = match encode_set_feed_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "set-feed", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "set-feed",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_share_list_shares_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "list-shares", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "list-shares",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// put-share (request/response).
    pub async fn put_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_req = match encode_share(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "put-share", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "put-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-share (request/response).
    pub async fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_share_delete_share_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "delete-share", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "delete-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }
}

/// Typed client for the GroupService service.
pub struct GroupAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> GroupAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-groups (request/response).
    pub async fn list_groups(&self, req: EmptyRequest) -> Result<GroupList, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "list-groups", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "list-groups",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-group (request/response).
    pub async fn get_group(&self, req: GroupID) -> Result<Group, ClientError> {
        let csil_req = match encode_group_get_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "get-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "get-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-group (request/response).
    pub async fn create_group(&self, req: CreateGroupRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_create_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "create-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "create-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-group (request/response).
    pub async fn update_group(&self, req: UpdateGroupRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_update_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "update-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "update-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-members (request/response).
    pub async fn set_members(&self, req: SetGroupMembersRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_set_group_members_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "set-members", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "set-members",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-group (request/response).
    pub async fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_delete_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "delete-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "delete-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DelegationService", "list-granted", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DelegationService",
            "list-granted",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-received (request/response).
    pub async fn list_received(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DelegationService", "list-received", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DelegationService",
            "list-received",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-delegation (request/response).
    pub async fn create_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_req = match encode_delegation(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DelegationService", "create-delegation", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DelegationService",
            "create-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-delegation (request/response).
    pub async fn update_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_req = match encode_delegation(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DelegationService", "update-delegation", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DelegationService",
            "update-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// revoke-delegation (request/response).
    pub async fn revoke_delegation(&self, req: DelegationID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_delegation_revoke_delegation_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DelegationService", "revoke-delegation", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DelegationService",
            "revoke-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_list_events_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-events", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "list-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_occurrence_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-event (request/response).
    pub async fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_req = match encode_event_get_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-occurrence (request/response).
    pub async fn get_occurrence(
        &self,
        req: GetOccurrenceRequest,
    ) -> Result<Occurrence, ClientError> {
        let csil_req = match encode_get_occurrence_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-occurrence", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-occurrence",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_occurrence(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-event (request/response).
    pub async fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_req = match encode_event(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "create-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "create-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-event (request/response).
    pub async fn update_event(&self, req: UpdateEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_update_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "update-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "update-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-event (request/response).
    pub async fn delete_event(
        &self,
        req: DeleteEventRequest,
    ) -> Result<DeleteEventResponse, ClientError> {
        let csil_req = match encode_delete_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "delete-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "delete-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delete_event_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// move-event (request/response).
    pub async fn move_event(&self, req: MoveEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_move_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "move-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "move-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// respond (request/response).
    pub async fn respond(&self, req: RespondRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_respond_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "respond", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "respond",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-invitations (request/response).
    pub async fn list_invitations(&self, req: EmptyRequest) -> Result<EventList, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-invitations", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "list-invitations",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// search-events (request/response).
    pub async fn search_events(&self, req: SearchEventsRequest) -> Result<EventList, ClientError> {
        let csil_req = match encode_search_events_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "search-events", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "search-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-trash (request/response).
    pub async fn list_trash(&self, req: ListTrashRequest) -> Result<TrashList, ClientError> {
        let csil_req = match encode_list_trash_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-trash", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "list-trash",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_trash_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// restore-event (request/response).
    pub async fn restore_event(&self, req: RestoreEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_restore_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "restore-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "restore-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// import-event (request/response).
    pub async fn import_event(&self, req: ImportEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_import_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "import-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "import-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-event-once (request/response).
    pub async fn create_event_once(
        &self,
        req: CreateEventOnceRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_req = match encode_create_event_once_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "create-event-once", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "create-event-once",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-create-receipt (request/response).
    pub async fn get_create_receipt(
        &self,
        req: CreateReceiptRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_req = match encode_create_receipt_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-create-receipt", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-create-receipt",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// split-series-once (request/response).
    pub async fn split_series_once(
        &self,
        req: SplitSeriesOnceRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_req = match encode_split_series_once_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "split-series-once", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "split-series-once",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-split-receipt (request/response).
    pub async fn get_split_receipt(
        &self,
        req: SplitReceiptRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_req = match encode_split_receipt_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-split-receipt", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-split-receipt",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_free_busy_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("FreeBusyService", "query", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "FreeBusyService",
            "query",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_free_busy_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_import_ics_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "import-ics", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "IcsService",
            "import-ics",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_import_result(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// export-calendar (request/response).
    pub async fn export_calendar(
        &self,
        req: ExportCalendarRequest,
    ) -> Result<IcsDocument, ClientError> {
        let csil_req = match encode_export_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "export-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "IcsService",
            "export-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// export-event (request/response).
    pub async fn export_event(&self, req: ExportEventRequest) -> Result<IcsDocument, ClientError> {
        let csil_req = match encode_export_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "export-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "IcsService",
            "export-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_audit_query(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuditService", "list-audit", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuditService",
            "list-audit",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_audit_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }
}
