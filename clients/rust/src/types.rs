//! Generated types from CSIL specification

#![allow(non_camel_case_types, clippy::large_enum_variant)]

/// Returned by a generated `validate` method when a field violates one of its
/// CSIL constraints. `field` names the offending field; `message` explains.
#[derive(Debug, Clone, PartialEq)]
pub struct ValidationError {
    pub field: String,
    pub message: String,
}

impl std::fmt::Display for ValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "validation failed for `{}`: {}",
            self.field, self.message
        )
    }
}

impl std::error::Error for ValidationError {}

pub type UserID = String;

pub type CalendarID = String;

pub type EventID = String;

pub type ShareID = String;

pub type DelegationID = String;

pub type AuditID = String;

pub type SessionID = String;

pub type Timestamp = String;

pub type DateText = String;

/// ShareRole variants
#[derive(Debug, Clone, PartialEq)]
pub enum ShareRole {
    FreeBusy,
    Read,
    Write,
    Manage,
}

/// AccessRole variants
#[derive(Debug, Clone, PartialEq)]
pub enum AccessRole {
    None,
    FreeBusy,
    Read,
    Write,
    Manage,
    Owner,
}

/// AccessPath variants
#[derive(Debug, Clone, PartialEq)]
pub enum AccessPath {
    Owner,
    Share,
    Delegation,
    Attendee,
}

/// Scope variants
#[derive(Debug, Clone, PartialEq)]
pub enum Scope {
    Calendars.read,
    Freebusy.read,
    Events.write,
    Events.respond,
    Calendars.manage,
}

/// CalendarKind variants
#[derive(Debug, Clone, PartialEq)]
pub enum CalendarKind {
    Local,
    Subscription,
}

/// EventStatus variants
#[derive(Debug, Clone, PartialEq)]
pub enum EventStatus {
    Confirmed,
    Tentative,
    Cancelled,
}

/// EventTransparency variants
#[derive(Debug, Clone, PartialEq)]
pub enum EventTransparency {
    Opaque,
    Transparent,
}

/// EventVisibility variants
#[derive(Debug, Clone, PartialEq)]
pub enum EventVisibility {
    Default,
    Public,
    Private,
    Confidential,
}

/// ParticipationStatus variants
#[derive(Debug, Clone, PartialEq)]
pub enum ParticipationStatus {
    NeedsAction,
    Accepted,
    Declined,
    Tentative,
    Delegated,
}

/// AttendeeRole variants
#[derive(Debug, Clone, PartialEq)]
pub enum AttendeeRole {
    Chair,
    Required,
    Optional,
    NonParticipant,
}

/// ReminderMethod variants
#[derive(Debug, Clone, PartialEq)]
pub enum ReminderMethod {
    Display,
    Email,
}

/// RecurrenceScope variants
#[derive(Debug, Clone, PartialEq)]
pub enum RecurrenceScope {
    All,
    This,
    ThisAndFuture,
}

/// ActingVia variants
#[derive(Debug, Clone, PartialEq)]
pub enum ActingVia {
    Self,
    Delegation,
    ActAsGrant,
}

#[derive(Debug, Clone, PartialEq)]
pub struct IdentityRef {
    pub domain: String,
    pub user_id: Option<String>,
    pub handle: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct User {
    pub user_id: UserID,
    pub domain: String,
    pub subject: String,
    pub handle: Option<String>,
    pub display_name: Option<String>,
    pub email: Option<String>,
    pub timezone: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PersonSummary {
    pub user_id: UserID,
    pub domain: String,
    pub subject: String,
    pub handle: Option<String>,
    pub display_name: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Calendar {
    pub calendar_id: CalendarID,
    pub owner: PersonSummary,
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub color: Option<String>,
    pub timezone: Option<String>,
    pub kind: CalendarKind,
    pub source_url: Option<String>,
    pub last_synced_at: Option<Timestamp>,
    pub sync_error: Option<String>,
    pub is_default: bool,
    pub access: AccessRole,
    pub access_path: AccessPath,
    pub feed_path: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Calendar {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.is_empty() || v.len() > 256usize {
                return Err(ValidationError {
                    field: "name".to_string(),
                    message: "length must be in 1..=256".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct CreateCalendarRequest {
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub color: Option<String>,
    pub timezone: Option<String>,
}

impl CreateCalendarRequest {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.is_empty() || v.len() > 256usize {
                return Err(ValidationError {
                    field: "name".to_string(),
                    message: "length must be in 1..=256".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct UpdateCalendarRequest {
    pub calendar_id: CalendarID,
    /// constraint: size in 1..=256
    pub name: Option<String>,
    pub description: Option<String>,
    pub color: Option<String>,
    pub timezone: Option<String>,
}

impl UpdateCalendarRequest {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        if let Some(v) = &self.name {
            if v.is_empty() || v.len() > 256usize {
                return Err(ValidationError {
                    field: "name".to_string(),
                    message: "length must be in 1..=256".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SubscribeCalendarRequest {
    /// constraint: size in 1..=256
    pub name: String,
    pub source_url: String,
    pub color: Option<String>,
}

impl SubscribeCalendarRequest {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.is_empty() || v.len() > 256usize {
                return Err(ValidationError {
                    field: "name".to_string(),
                    message: "length must be in 1..=256".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct SetFeedRequest {
    pub calendar_id: CalendarID,
    pub enabled: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CalendarList {
    pub calendars: Vec<Calendar>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Share {
    pub share_id: ShareID,
    pub calendar_id: CalendarID,
    pub grantee: IdentityRef,
    pub role: ShareRole,
    pub grantee_person: Option<PersonSummary>,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ShareList {
    pub shares: Vec<Share>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Delegation {
    pub delegation_id: DelegationID,
    pub principal: PersonSummary,
    pub delegate: IdentityRef,
    pub delegate_person: Option<PersonSummary>,
    pub scopes: Vec<Scope>,
    pub calendar_ids: Option<Vec<CalendarID>>,
    pub note: Option<String>,
    pub expires_at: Option<Timestamp>,
    pub created_at: Timestamp,
    pub revoked_at: Option<Timestamp>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DelegationList {
    pub delegations: Vec<Delegation>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AssumePrincipalRequest {
    pub principal_user_id: UserID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Recurrence {
    pub rrule: String,
    pub exdates: Option<Vec<Timestamp>>,
    pub rdates: Option<Vec<Timestamp>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Attendee {
    pub identity: Option<IdentityRef>,
    pub email: Option<String>,
    pub name: Option<String>,
    /// default: "required"
    pub role: Option<AttendeeRole>,
    /// default: "needs_action"
    pub status: Option<ParticipationStatus>,
    /// default: true
    pub rsvp: Option<bool>,
    pub person: Option<PersonSummary>,
    pub responded_by: Option<PersonSummary>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Reminder {
    pub minutes_before: i64,
    /// default: "display"
    pub method: Option<ReminderMethod>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Event {
    pub event_id: EventID,
    pub calendar_id: CalendarID,
    pub uid: Option<String>,
    /// constraint: size in 1..=1024
    pub title: String,
    pub description: Option<String>,
    pub location: Option<String>,
    pub url: Option<String>,
    pub conference_url: Option<String>,
    pub start: Option<Timestamp>,
    pub end: Option<Timestamp>,
    /// default: false
    pub all_day: Option<bool>,
    pub start_date: Option<DateText>,
    pub end_date: Option<DateText>,
    pub timezone: Option<String>,
    /// default: "confirmed"
    pub status: Option<EventStatus>,
    /// default: "opaque"
    pub transparency: Option<EventTransparency>,
    /// default: "default"
    pub visibility: Option<EventVisibility>,
    pub recurrence: Option<Recurrence>,
    pub master_event_id: Option<EventID>,
    pub recurrence_id: Option<Timestamp>,
    pub attendees: Option<Vec<Attendee>>,
    pub reminders: Option<Vec<Reminder>>,
    pub categories: Option<Vec<String>>,
    pub color: Option<String>,
    pub organizer: Option<PersonSummary>,
    pub created_by: Option<PersonSummary>,
    pub updated_by: Option<PersonSummary>,
    pub updated_via: Option<ActingVia>,
    pub sequence: i64,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Event {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.title;
            if v.is_empty() || v.len() > 1024usize {
                return Err(ValidationError {
                    field: "title".to_string(),
                    message: "length must be in 1..=1024".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Occurrence {
    pub event: Event,
    pub start: Timestamp,
    pub end: Timestamp,
    pub recurrence_id: Option<Timestamp>,
    pub access: AccessRole,
    pub busy_only: bool,
    pub invited: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ListEventsRequest {
    pub range_start: Timestamp,
    pub range_end: Timestamp,
    pub calendar_ids: Option<Vec<CalendarID>>,
    /// default: true
    pub include_invitations: Option<bool>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct OccurrenceList {
    pub occurrences: Vec<Occurrence>,
    pub truncated: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct UpdateEventRequest {
    pub event: Event,
    /// default: "all"
    pub scope: Option<RecurrenceScope>,
    pub recurrence_id: Option<Timestamp>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DeleteEventRequest {
    pub event_id: EventID,
    /// default: "all"
    pub scope: Option<RecurrenceScope>,
    pub recurrence_id: Option<Timestamp>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MoveEventRequest {
    pub event_id: EventID,
    pub calendar_id: CalendarID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RespondRequest {
    pub event_id: EventID,
    pub status: ParticipationStatus,
    pub recurrence_id: Option<Timestamp>,
    pub comment: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct SearchEventsRequest {
    /// constraint: size in 1..=256
    pub query: String,
    /// default: 50
    pub limit: Option<u64>,
}

impl SearchEventsRequest {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.query;
            if v.is_empty() || v.len() > 256usize {
                return Err(ValidationError {
                    field: "query".to_string(),
                    message: "length must be in 1..=256".to_string(),
                });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct EventList {
    pub events: Vec<Event>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BusyInterval {
    pub start: Timestamp,
    pub end: Timestamp,
    pub tentative: Option<bool>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct FreeBusyRequest {
    pub people: Vec<IdentityRef>,
    pub range_start: Timestamp,
    pub range_end: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PersonFreeBusy {
    pub identity: IdentityRef,
    pub person: Option<PersonSummary>,
    pub busy: Vec<BusyInterval>,
    pub visible: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct FreeBusyResponse {
    pub people: Vec<PersonFreeBusy>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ImportIcsRequest {
    pub calendar_id: CalendarID,
    pub ics: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ImportResult {
    pub created: u64,
    pub updated: u64,
    pub skipped: u64,
    pub warnings: Option<Vec<String>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ExportCalendarRequest {
    pub calendar_id: CalendarID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ExportEventRequest {
    pub event_id: EventID,
    pub method: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct IcsDocument {
    pub ics: String,
    pub filename: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuditEntry {
    pub audit_id: AuditID,
    pub at: Timestamp,
    pub actor: PersonSummary,
    pub principal: PersonSummary,
    pub via: ActingVia,
    pub application: Option<String>,
    pub operation: String,
    pub target: Option<String>,
    pub outcome: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuditQuery {
    /// default: 100
    pub limit: Option<u64>,
    /// default: false
    pub delegated_only: Option<bool>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuditList {
    pub entries: Vec<AuditEntry>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CompleteRequest {
    pub encrypted_token: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct LoginResponse {
    pub token: String,
    pub expires_at: Timestamp,
    pub user: PersonSummary,
    pub acting_as: Option<PersonSummary>,
    pub via: ActingVia,
    pub scopes: Option<Vec<Scope>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MeResponse {
    pub user: User,
    pub acting_as: Option<PersonSummary>,
    pub via: ActingVia,
    pub scopes: Option<Vec<Scope>>,
    pub principals: Vec<PersonSummary>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ExchangeActAsCredentialRequest {
    pub credential: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ScopeSetRequest {
    pub grantee_domain: String,
    pub grantee_user_id: String,
    pub grantee_application_id: String,
    pub languages: Option<Vec<String>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ScopeSetResponse {
    pub signed_scope_set: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevUser {
    pub user_id: UserID,
    pub handle: Option<String>,
    pub display_name: Option<String>,
    pub domain: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevUsersResponse {
    pub users: Vec<DevUser>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevLoginRequest {
    pub user_id: Option<UserID>,
    pub handle: Option<String>,
    pub display_name: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct LookupPersonRequest {
    pub identity: IdentityRef,
}

#[derive(Debug, Clone, PartialEq)]
pub struct UpdateProfileRequest {
    pub display_name: Option<String>,
    pub timezone: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct EmptyRequest {}

#[derive(Debug, Clone, PartialEq)]
pub struct EmptyResponse {}

#[derive(Debug, Clone, PartialEq)]
pub struct ServiceError {
    pub code: u64,
    pub message: String,
}
