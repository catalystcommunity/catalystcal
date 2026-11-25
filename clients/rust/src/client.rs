//! Generated transport-agnostic service clients from CSIL specification

use super::codec::*;
use super::types::*;

/// Error from a generated client call: a structured error the service returned,
/// or a transport-level failure. The caller-supplied `Transport` decides how an
/// error response maps onto `Service`. `Encode` means the request has no valid
/// wire form, so the client did not call the transport.
#[derive(Debug, Clone)]
pub enum ClientError {
    Service { code: i64, message: String },
    Transport(String),
    Encode(String),
    ApplicationServiceError(Box<ServiceError>),
    Decode(String),
}

impl std::fmt::Display for ClientError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ClientError::Service { code, message } => write!(f, "service error {code}: {message}"),
            Self::Encode(msg) => write!(f, "invalid request: {msg}"),
            Self::ApplicationServiceError(e) => write!(f, "application error: {e:?}"),
            Self::Decode(msg) => write!(f, "invalid reply: {msg}"),
            ClientError::Transport(msg) => write!(f, "transport error: {msg}"),
        }
    }
}

impl std::error::Error for ClientError {}

/// A status-zero CSIL-RPC reply: `variant` names the declared output arm that
/// `payload` encodes. A transport returns it from `call_reply`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WireReply {
    pub variant: Option<String>,
    pub payload: Vec<u8>,
}

/// The caller-supplied byte carrier: it performs the call named by `(service, op)`
/// with the already-encoded request bytes and returns the response bytes, or an
/// error. The generated client owns (de)serialization via the codec; the carrier
/// only moves bytes, so it can be HTTP, a queue, or an in-process loop.
pub trait Transport {
    fn call(&self, service: &str, op: &str, req: &[u8]) -> Result<Vec<u8>, ClientError>;

    /// Perform a call whose operation declares several reply arms and return
    /// the status-zero reply `variant` with its payload, so the client can
    /// return the success arm or the declared error arm.
    fn call_reply(&self, service: &str, op: &str, req: &[u8]) -> Result<WireReply, ClientError>;
}
impl From<CsilCborError> for ClientError {
    fn from(error: CsilCborError) -> Self {
        Self::Decode(error.0)
    }
}

/// Resolve a status-zero RPC reply into its success payload or its declared error
/// arm. Generated clients call it with the reply from `Transport::call_reply`.
pub fn resolve_rpc_reply(
    service: &str,
    op: &str,
    variant: Option<&str>,
    payload: Vec<u8>,
) -> Result<Vec<u8>, ClientError> {
    match (service, op) {
        ("AuthService", "complete") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "resume-session") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "end-session") => match variant {
            Some("EmptyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "me") => match variant {
            Some("MeResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "assume-principal") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "release-principal") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "exchange-act-as-credential") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "get-act-as-scope-set") => match variant {
            Some("ScopeSetResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "get-application-identity") => match variant {
            Some("ApplicationIdentityResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuthService", "exchange-application-presentation") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DevAuthService", "list-dev-users") => match variant {
            Some("DevUsersResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DevAuthService", "dev-login") => match variant {
            Some("LoginResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("UserService", "get-profile") => match variant {
            Some("User") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("UserService", "update-profile") => match variant {
            Some("User") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("UserService", "lookup-person") => match variant {
            Some("PersonSummary") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "list-calendars") => match variant {
            Some("CalendarList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "get-calendar") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "create-calendar") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "update-calendar") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "delete-calendar") => match variant {
            Some("EmptyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "subscribe-calendar") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "refresh-subscription") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("CalendarService", "set-feed") => match variant {
            Some("Calendar") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("ShareService", "list-shares") => match variant {
            Some("ShareList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("ShareService", "put-share") => match variant {
            Some("Share") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("ShareService", "delete-share") => match variant {
            Some("EmptyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "list-groups") => match variant {
            Some("GroupList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "get-group") => match variant {
            Some("Group") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "create-group") => match variant {
            Some("Group") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "update-group") => match variant {
            Some("Group") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "set-members") => match variant {
            Some("Group") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("GroupService", "delete-group") => match variant {
            Some("EmptyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DelegationService", "list-granted") => match variant {
            Some("DelegationList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DelegationService", "list-received") => match variant {
            Some("DelegationList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DelegationService", "create-delegation") => match variant {
            Some("Delegation") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DelegationService", "update-delegation") => match variant {
            Some("Delegation") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("DelegationService", "revoke-delegation") => match variant {
            Some("EmptyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "list-events") => match variant {
            Some("OccurrenceList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "get-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "get-occurrence") => match variant {
            Some("Occurrence") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "create-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "update-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "delete-event") => match variant {
            Some("DeleteEventResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "move-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "respond") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "list-invitations") => match variant {
            Some("EventList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "search-events") => match variant {
            Some("EventList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "list-trash") => match variant {
            Some("TrashList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "restore-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "import-event") => match variant {
            Some("Event") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "create-event-once") => match variant {
            Some("CreateEventReceipt") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "get-create-receipt") => match variant {
            Some("CreateEventReceipt") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "split-series-once") => match variant {
            Some("SplitSeriesReceipt") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("EventService", "get-split-receipt") => match variant {
            Some("SplitSeriesReceipt") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("FreeBusyService", "query") => match variant {
            Some("FreeBusyResponse") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("IcsService", "import-ics") => match variant {
            Some("ImportResult") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("IcsService", "export-calendar") => match variant {
            Some("IcsDocument") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("IcsService", "export-event") => match variant {
            Some("IcsDocument") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        ("AuditService", "list-audit") => match variant {
            Some("AuditList") => Ok(payload),
            Some("ServiceError") => {
                let value = decode_service_error(&payload)?;
                let value = Box::new(value);
                let error = ClientError::ApplicationServiceError(value);
                Err(error)
            }
            _ => Err(ClientError::Decode(format!(
                "invalid reply variant {variant:?}"
            ))),
        },
        _ => Ok(payload),
    }
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
        let csil_req = match encode_complete_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "complete", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "complete",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// resume-session (request/response).
    pub fn resume_session(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "resume-session", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "resume-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// end-session (request/response).
    pub fn end_session(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "end-session", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "end-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// me (request/response).
    pub fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self.transport.call_reply("AuthService", "me", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "me",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// assume-principal (request/response).
    pub fn assume_principal(
        &self,
        req: AssumePrincipalRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_assume_principal_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "assume-principal", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "assume-principal",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// release-principal (request/response).
    pub fn release_principal(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("AuthService", "release-principal", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "release-principal",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-act-as-credential (request/response).
    pub fn exchange_act_as_credential(
        &self,
        req: ExchangeActAsCredentialRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_exchange_act_as_credential_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("AuthService", "exchange-act-as-credential", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "exchange-act-as-credential",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-act-as-scope-set (request/response).
    pub fn get_act_as_scope_set(
        &self,
        req: ScopeSetRequest,
    ) -> Result<ScopeSetResponse, ClientError> {
        let csil_req = match encode_scope_set_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("AuthService", "get-act-as-scope-set", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "get-act-as-scope-set",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_scope_set_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-application-identity (request/response).
    pub fn get_application_identity(
        &self,
        req: EmptyRequest,
    ) -> Result<ApplicationIdentityResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("AuthService", "get-application-identity", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "get-application-identity",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_application_identity_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-application-presentation (request/response).
    pub fn exchange_application_presentation(
        &self,
        req: ExchangeApplicationPresentationRequest,
    ) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_exchange_application_presentation_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self.transport.call_reply(
            "AuthService",
            "exchange-application-presentation",
            &csil_req,
        )?;
        let csil_resp = resolve_rpc_reply(
            "AuthService",
            "exchange-application-presentation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DevAuthService", "list-dev-users", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DevAuthService",
            "list-dev-users",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// dev-login (request/response).
    pub fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_dev_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DevAuthService", "dev-login", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DevAuthService",
            "dev-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "get-profile", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "UserService",
            "get-profile",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_user(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-profile (request/response).
    pub fn update_profile(&self, req: UpdateProfileRequest) -> Result<User, ClientError> {
        let csil_req = match encode_update_profile_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "update-profile", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "UserService",
            "update-profile",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_user(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// lookup-person (request/response).
    pub fn lookup_person(&self, req: LookupPersonRequest) -> Result<PersonSummary, ClientError> {
        let csil_req = match encode_lookup_person_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("UserService", "lookup-person", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "UserService",
            "lookup-person",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_person_summary(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "list-calendars", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "list-calendars",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-calendar (request/response).
    pub fn get_calendar(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_req = match encode_calendar_get_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "get-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "get-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-calendar (request/response).
    pub fn create_calendar(&self, req: CreateCalendarRequest) -> Result<Calendar, ClientError> {
        let csil_req = match encode_create_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "create-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "create-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-calendar (request/response).
    pub fn update_calendar(&self, req: UpdateCalendarRequest) -> Result<Calendar, ClientError> {
        let csil_req = match encode_update_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "update-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "update-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-calendar (request/response).
    pub fn delete_calendar(&self, req: CalendarID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_calendar_delete_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "delete-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "delete-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// subscribe-calendar (request/response).
    pub fn subscribe_calendar(
        &self,
        req: SubscribeCalendarRequest,
    ) -> Result<Calendar, ClientError> {
        let csil_req = match encode_subscribe_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "subscribe-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "subscribe-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// refresh-subscription (request/response).
    pub fn refresh_subscription(&self, req: CalendarID) -> Result<Calendar, ClientError> {
        let csil_req = match encode_calendar_refresh_subscription_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("CalendarService", "refresh-subscription", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "refresh-subscription",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-feed (request/response).
    pub fn set_feed(&self, req: SetFeedRequest) -> Result<Calendar, ClientError> {
        let csil_req = match encode_set_feed_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "set-feed", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "CalendarService",
            "set-feed",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_share_list_shares_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "list-shares", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "ShareService",
            "list-shares",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// put-share (request/response).
    pub fn put_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_req = match encode_share(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "put-share", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "ShareService",
            "put-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-share (request/response).
    pub fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_share_delete_share_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "delete-share", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "ShareService",
            "delete-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "list-groups", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "list-groups",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-group (request/response).
    pub fn get_group(&self, req: GroupID) -> Result<Group, ClientError> {
        let csil_req = match encode_group_get_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "get-group", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "get-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-group (request/response).
    pub fn create_group(&self, req: CreateGroupRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_create_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "create-group", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "create-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-group (request/response).
    pub fn update_group(&self, req: UpdateGroupRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_update_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "update-group", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "update-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-members (request/response).
    pub fn set_members(&self, req: SetGroupMembersRequest) -> Result<Group, ClientError> {
        let csil_req = match encode_set_group_members_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "set-members", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "set-members",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-group (request/response).
    pub fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_delete_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "delete-group", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "GroupService",
            "delete-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DelegationService", "list-granted", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DelegationService",
            "list-granted",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-received (request/response).
    pub fn list_received(&self, req: EmptyRequest) -> Result<DelegationList, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DelegationService", "list-received", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DelegationService",
            "list-received",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-delegation (request/response).
    pub fn create_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_req = match encode_delegation(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DelegationService", "create-delegation", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DelegationService",
            "create-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-delegation (request/response).
    pub fn update_delegation(&self, req: Delegation) -> Result<Delegation, ClientError> {
        let csil_req = match encode_delegation(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DelegationService", "update-delegation", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DelegationService",
            "update-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delegation(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// revoke-delegation (request/response).
    pub fn revoke_delegation(&self, req: DelegationID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_delegation_revoke_delegation_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("DelegationService", "revoke-delegation", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "DelegationService",
            "revoke-delegation",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_list_events_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-events", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "list-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_occurrence_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-event (request/response).
    pub fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_req = match encode_event_get_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "get-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-occurrence (request/response).
    pub fn get_occurrence(&self, req: GetOccurrenceRequest) -> Result<Occurrence, ClientError> {
        let csil_req = match encode_get_occurrence_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-occurrence", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "get-occurrence",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_occurrence(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-event (request/response).
    pub fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_req = match encode_event(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "create-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "create-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-event (request/response).
    pub fn update_event(&self, req: UpdateEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_update_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "update-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "update-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-event (request/response).
    pub fn delete_event(
        &self,
        req: DeleteEventRequest,
    ) -> Result<DeleteEventResponse, ClientError> {
        let csil_req = match encode_delete_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "delete-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "delete-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_delete_event_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// move-event (request/response).
    pub fn move_event(&self, req: MoveEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_move_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "move-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "move-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// respond (request/response).
    pub fn respond(&self, req: RespondRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_respond_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "respond", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "respond",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-invitations (request/response).
    pub fn list_invitations(&self, req: EmptyRequest) -> Result<EventList, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("EventService", "list-invitations", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "list-invitations",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// search-events (request/response).
    pub fn search_events(&self, req: SearchEventsRequest) -> Result<EventList, ClientError> {
        let csil_req = match encode_search_events_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "search-events", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "search-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-trash (request/response).
    pub fn list_trash(&self, req: ListTrashRequest) -> Result<TrashList, ClientError> {
        let csil_req = match encode_list_trash_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-trash", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "list-trash",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_trash_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// restore-event (request/response).
    pub fn restore_event(&self, req: RestoreEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_restore_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "restore-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "restore-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// import-event (request/response).
    pub fn import_event(&self, req: ImportEventRequest) -> Result<Event, ClientError> {
        let csil_req = match encode_import_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "import-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "import-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-event-once (request/response).
    pub fn create_event_once(
        &self,
        req: CreateEventOnceRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_req = match encode_create_event_once_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("EventService", "create-event-once", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "create-event-once",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-create-receipt (request/response).
    pub fn get_create_receipt(
        &self,
        req: CreateReceiptRequest,
    ) -> Result<CreateEventReceipt, ClientError> {
        let csil_req = match encode_create_receipt_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("EventService", "get-create-receipt", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "get-create-receipt",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_create_event_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// split-series-once (request/response).
    pub fn split_series_once(
        &self,
        req: SplitSeriesOnceRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_req = match encode_split_series_once_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("EventService", "split-series-once", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "split-series-once",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-split-receipt (request/response).
    pub fn get_split_receipt(
        &self,
        req: SplitReceiptRequest,
    ) -> Result<SplitSeriesReceipt, ClientError> {
        let csil_req = match encode_split_receipt_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply =
            self.transport
                .call_reply("EventService", "get-split-receipt", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "EventService",
            "get-split-receipt",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_split_series_receipt(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_free_busy_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("FreeBusyService", "query", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "FreeBusyService",
            "query",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_free_busy_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_import_ics_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "import-ics", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "IcsService",
            "import-ics",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_import_result(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// export-calendar (request/response).
    pub fn export_calendar(&self, req: ExportCalendarRequest) -> Result<IcsDocument, ClientError> {
        let csil_req = match encode_export_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "export-calendar", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "IcsService",
            "export-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// export-event (request/response).
    pub fn export_event(&self, req: ExportEventRequest) -> Result<IcsDocument, ClientError> {
        let csil_req = match encode_export_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("IcsService", "export-event", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "IcsService",
            "export-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_ics_document(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_audit_query(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuditService", "list-audit", &csil_req)?;
        let csil_resp = resolve_rpc_reply(
            "AuditService",
            "list-audit",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_audit_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }
}
