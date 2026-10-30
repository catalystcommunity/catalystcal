//! Generated self-contained canonical-CBOR codec from CSIL specification.
//!
//! CSIL is the CBOR Service Interface Language; this codec owns the payload
//! wire (a CBOR map keyed by the verbatim CSIL field name in canonical RFC
//! 8949 order) so the generated types need no serde derive. One
//! `encode_`/`decode_` pair is emitted per record type.
#![allow(dead_code, clippy::vec_init_then_push)]

use super::types::*;

/// A decode failure: the CBOR was malformed or did not match the expected shape.
#[derive(Debug, Clone, PartialEq)]
pub struct CsilCborError(pub String);

impl std::fmt::Display for CsilCborError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for CsilCborError {}

/// A minimal canonical-CBOR value tree: a closed set of variants the generated codec
/// builds and walks. A map is an ordered list of pairs, so the encoder controls the
/// wire order of a record's keys explicitly (laid down in canonical order).
#[derive(Debug, Clone, PartialEq)]
pub enum CsilCborValue {
    Uint(u64),
    Int(i64),
    Bool(bool),
    Float(f64),
    Null,
    Text(String),
    Bytes(Vec<u8>),
    Array(Vec<CsilCborValue>),
    Map(Vec<(CsilCborValue, CsilCborValue)>),
    Tag(u64, Box<CsilCborValue>),
}

fn cbor_int(x: i64) -> CsilCborValue {
    CsilCborValue::Int(x)
}
fn cbor_uint(x: u64) -> CsilCborValue {
    CsilCborValue::Uint(x)
}
fn cbor_float(x: f64) -> CsilCborValue {
    CsilCborValue::Float(x)
}
fn cbor_bool(x: bool) -> CsilCborValue {
    CsilCborValue::Bool(x)
}
fn cbor_text(x: &str) -> CsilCborValue {
    CsilCborValue::Text(x.to_string())
}
fn cbor_bytes(x: &[u8]) -> CsilCborValue {
    CsilCborValue::Bytes(x.to_vec())
}

/// Serialize a value tree to canonical CBOR bytes.
fn cbor_encode(v: &CsilCborValue) -> Vec<u8> {
    let mut out = Vec::new();
    cbor_enc(v, &mut out);
    out
}

fn cbor_head(major: u8, n: u64, out: &mut Vec<u8>) {
    let mt = major << 5;
    if n < 24 {
        out.push(mt | n as u8);
    } else if n < 0x100 {
        out.push(mt | 24);
        out.push(n as u8);
    } else if n < 0x10000 {
        out.push(mt | 25);
        out.extend_from_slice(&(n as u16).to_be_bytes());
    } else if n < 0x1_0000_0000 {
        out.push(mt | 26);
        out.extend_from_slice(&(n as u32).to_be_bytes());
    } else {
        out.push(mt | 27);
        out.extend_from_slice(&n.to_be_bytes());
    }
}

fn cbor_enc(v: &CsilCborValue, out: &mut Vec<u8>) {
    match v {
        CsilCborValue::Uint(x) => cbor_head(0, *x, out),
        // A non-negative `Int` rides major type 0 so it is byte-identical to a `Uint`
        // of the same magnitude; only a genuinely negative value uses major type 1.
        CsilCborValue::Int(x) => {
            if *x >= 0 {
                cbor_head(0, *x as u64, out);
            } else {
                cbor_head(1, (-(*x + 1)) as u64, out);
            }
        }
        CsilCborValue::Bool(x) => out.push(if *x { 0xf5 } else { 0xf4 }),
        CsilCborValue::Null => out.push(0xf6),
        CsilCborValue::Float(x) => {
            out.push(0xfb);
            out.extend_from_slice(&x.to_bits().to_be_bytes());
        }
        CsilCborValue::Text(s) => {
            let bytes = s.as_bytes();
            cbor_head(3, bytes.len() as u64, out);
            out.extend_from_slice(bytes);
        }
        CsilCborValue::Bytes(b) => {
            cbor_head(2, b.len() as u64, out);
            out.extend_from_slice(b);
        }
        CsilCborValue::Array(items) => {
            cbor_head(4, items.len() as u64, out);
            for item in items {
                cbor_enc(item, out);
            }
        }
        CsilCborValue::Map(entries) => {
            cbor_head(5, entries.len() as u64, out);
            for (k, val) in entries {
                cbor_enc(k, out);
                cbor_enc(val, out);
            }
        }
        CsilCborValue::Tag(num, inner) => {
            cbor_head(6, *num, out);
            cbor_enc(inner, out);
        }
    }
}

/// Parse a full CBOR item and reject trailing bytes, so a payload that is not
/// exactly one value is an error rather than a silently-truncated read.
fn cbor_decode(b: &[u8]) -> Result<CsilCborValue, CsilCborError> {
    let mut pos = 0usize;
    let v = cbor_dec(b, &mut pos, 0)?;
    if pos != b.len() {
        return Err(CsilCborError(format!(
            "csil cbor: {} trailing bytes",
            b.len() - pos
        )));
    }
    Ok(v)
}

fn cbor_read_arg(b: &[u8], pos: &mut usize, low: u8) -> Result<u64, CsilCborError> {
    if low < 24 {
        *pos += 1;
        return Ok(low as u64);
    }
    let width = match low {
        24 => 1usize,
        25 => 2,
        26 => 4,
        27 => 8,
        _ => {
            return Err(CsilCborError(format!(
                "csil cbor: reserved additional info {low}"
            )))
        }
    };
    if *pos >= b.len() || width > b.len() - *pos - 1 {
        return Err(CsilCborError("csil cbor: truncated argument".to_string()));
    }
    let mut v = 0u64;
    for &byte in &b[*pos + 1..*pos + 1 + width] {
        v = (v << 8) | byte as u64;
    }
    *pos += 1 + width;
    Ok(v)
}

/// The most elements a decoded array or map reserves before it reads them. The
/// declared length is checked against the remaining input, but one input byte can
/// become a much larger value, so reserving the full declared length lets a small
/// frame reserve a large multiple of its size at every nesting level. Past this
/// bound, the vector grows only as elements are actually read.
const CSIL_CBOR_PREALLOC_LIMIT: usize = 1024;

fn cbor_dec(b: &[u8], pos: &mut usize, depth: usize) -> Result<CsilCborValue, CsilCborError> {
    if depth > 64 {
        return Err(CsilCborError(
            "csil cbor: nesting limit exceeded".to_string(),
        ));
    }
    if *pos >= b.len() {
        return Err(CsilCborError(
            "csil cbor: unexpected end of input".to_string(),
        ));
    }
    let ib = b[*pos];
    let major = ib >> 5;
    let low = ib & 0x1f;
    if major == 7 {
        return match low {
            20 => {
                *pos += 1;
                Ok(CsilCborValue::Bool(false))
            }
            21 => {
                *pos += 1;
                Ok(CsilCborValue::Bool(true))
            }
            22 | 23 => {
                *pos += 1;
                Ok(CsilCborValue::Null)
            }
            26 => {
                let bits = cbor_read_arg(b, pos, low)?;
                Ok(CsilCborValue::Float(f32::from_bits(bits as u32) as f64))
            }
            27 => {
                let bits = cbor_read_arg(b, pos, low)?;
                Ok(CsilCborValue::Float(f64::from_bits(bits)))
            }
            _ => Err(CsilCborError(format!(
                "csil cbor: unsupported simple value {low}"
            ))),
        };
    }
    let arg = cbor_read_arg(b, pos, low)?;
    match major {
        0 => Ok(CsilCborValue::Uint(arg)),
        1 => {
            if arg > i64::MAX as u64 {
                return Err(CsilCborError(
                    "csil cbor: negative integer out of range".to_string(),
                ));
            }
            Ok(CsilCborValue::Int(-1 - arg as i64))
        }
        2 => {
            if arg > (b.len() - *pos) as u64 {
                return Err(CsilCborError(
                    "csil cbor: truncated byte string".to_string(),
                ));
            }
            let n = arg as usize;
            let slice = b[*pos..*pos + n].to_vec();
            *pos += n;
            Ok(CsilCborValue::Bytes(slice))
        }
        3 => {
            if arg > (b.len() - *pos) as u64 {
                return Err(CsilCborError(
                    "csil cbor: truncated text string".to_string(),
                ));
            }
            let n = arg as usize;
            let s = std::str::from_utf8(&b[*pos..*pos + n])
                .map_err(|e| CsilCborError(format!("csil cbor: invalid utf-8: {e}")))?
                .to_string();
            *pos += n;
            Ok(CsilCborValue::Text(s))
        }
        4 => {
            if arg > (b.len() - *pos) as u64 {
                return Err(CsilCborError(
                    "csil cbor: array length exceeds remaining input".to_string(),
                ));
            }
            let n = arg as usize;
            let mut items = Vec::with_capacity(n.min(CSIL_CBOR_PREALLOC_LIMIT));
            for _ in 0..n {
                items.push(cbor_dec(b, pos, depth + 1)?);
            }
            Ok(CsilCborValue::Array(items))
        }
        5 => {
            if arg > (b.len() - *pos) as u64 {
                return Err(CsilCborError(
                    "csil cbor: map length exceeds remaining input".to_string(),
                ));
            }
            let n = arg as usize;
            let mut entries = Vec::with_capacity(n.min(CSIL_CBOR_PREALLOC_LIMIT));
            for _ in 0..n {
                let k = cbor_dec(b, pos, depth + 1)?;
                let val = cbor_dec(b, pos, depth + 1)?;
                entries.push((k, val));
            }
            Ok(CsilCborValue::Map(entries))
        }
        6 => {
            let inner = cbor_dec(b, pos, depth + 1)?;
            Ok(CsilCborValue::Tag(arg, Box::new(inner)))
        }
        _ => Err(CsilCborError(format!(
            "csil cbor: unexpected major type {major}"
        ))),
    }
}

/// Map a typed slice to a CBOR array via the per-element encoder.
fn cbor_enc_array<E>(xs: &[E], f: impl Fn(&E) -> CsilCborValue) -> CsilCborValue {
    CsilCborValue::Array(xs.iter().map(f).collect())
}

/// Map a typed map to a CBOR map. Rust `HashMap` iteration is unordered, so the inner
/// map's entry order is not canonicalized; the record's own keys (laid down at
/// generation time) are what the cross-language wire contract pins.
fn cbor_enc_map<K, V>(
    m: &std::collections::HashMap<K, V>,
    kf: impl Fn(&K) -> CsilCborValue,
    vf: impl Fn(&V) -> CsilCborValue,
) -> CsilCborValue {
    CsilCborValue::Map(m.iter().map(|(k, v)| (kf(k), vf(v))).collect())
}

fn cbor_dec_array<E>(
    v: &CsilCborValue,
    f: impl Fn(&CsilCborValue) -> Result<E, CsilCborError>,
) -> Result<Vec<E>, CsilCborError> {
    cbor_as_array(v)?.iter().map(f).collect()
}

fn cbor_dec_map<K: std::cmp::Eq + std::hash::Hash, V>(
    v: &CsilCborValue,
    kf: impl Fn(&CsilCborValue) -> Result<K, CsilCborError>,
    vf: impl Fn(&CsilCborValue) -> Result<V, CsilCborError>,
) -> Result<std::collections::HashMap<K, V>, CsilCborError> {
    let entries = cbor_as_map(v)?;
    let mut out = std::collections::HashMap::with_capacity(entries.len());
    for (k, val) in entries {
        out.insert(kf(k)?, vf(val)?);
    }
    Ok(out)
}

fn cbor_map_get<'a>(v: &'a CsilCborValue, key: &str) -> Option<&'a CsilCborValue> {
    if let CsilCborValue::Map(entries) = v {
        for (k, val) in entries {
            if matches!(k, CsilCborValue::Text(name) if name == key) {
                return Some(val);
            }
        }
    }
    None
}

fn cbor_expect_value(v: &CsilCborValue, expected: &CsilCborValue) -> Result<(), CsilCborError> {
    if v == expected {
        Ok(())
    } else {
        Err(CsilCborError(format!(
            "csil cbor: expected literal {expected:?}, got {v:?}"
        )))
    }
}

fn cbor_require<'a>(v: &'a CsilCborValue, key: &str) -> Result<&'a CsilCborValue, CsilCborError> {
    cbor_map_get(v, key).ok_or_else(|| CsilCborError(format!("csil cbor: missing field {key:?}")))
}

fn cbor_as_i64(v: &CsilCborValue) -> Result<i64, CsilCborError> {
    match v {
        CsilCborValue::Uint(x) => i64::try_from(*x)
            .map_err(|_| CsilCborError("csil cbor: integer overflows i64".to_string())),
        CsilCborValue::Int(x) => Ok(*x),
        _ => Err(CsilCborError("csil cbor: expected integer".to_string())),
    }
}

fn cbor_as_u64(v: &CsilCborValue) -> Result<u64, CsilCborError> {
    match v {
        CsilCborValue::Uint(x) => Ok(*x),
        CsilCborValue::Int(x) if *x >= 0 => Ok(*x as u64),
        CsilCborValue::Int(_) => Err(CsilCborError(
            "csil cbor: negative integer where unsigned expected".to_string(),
        )),
        _ => Err(CsilCborError(
            "csil cbor: expected unsigned integer".to_string(),
        )),
    }
}

fn cbor_as_f64(v: &CsilCborValue) -> Result<f64, CsilCborError> {
    match v {
        CsilCborValue::Float(x) => Ok(*x),
        CsilCborValue::Uint(x) => Ok(*x as f64),
        CsilCborValue::Int(x) => Ok(*x as f64),
        _ => Err(CsilCborError("csil cbor: expected float".to_string())),
    }
}

fn cbor_as_bool(v: &CsilCborValue) -> Result<bool, CsilCborError> {
    match v {
        CsilCborValue::Bool(b) => Ok(*b),
        _ => Err(CsilCborError("csil cbor: expected bool".to_string())),
    }
}

fn cbor_as_text(v: &CsilCborValue) -> Result<String, CsilCborError> {
    match v {
        CsilCborValue::Text(s) => Ok(s.clone()),
        _ => Err(CsilCborError("csil cbor: expected text".to_string())),
    }
}

fn cbor_as_bytes(v: &CsilCborValue) -> Result<Vec<u8>, CsilCborError> {
    match v {
        CsilCborValue::Bytes(b) => Ok(b.clone()),
        _ => Err(CsilCborError("csil cbor: expected byte string".to_string())),
    }
}

fn cbor_as_array(v: &CsilCborValue) -> Result<&[CsilCborValue], CsilCborError> {
    match v {
        CsilCborValue::Array(a) => Ok(a),
        _ => Err(CsilCborError("csil cbor: expected array".to_string())),
    }
}

fn cbor_as_map(v: &CsilCborValue) -> Result<&[(CsilCborValue, CsilCborValue)], CsilCborError> {
    match v {
        CsilCborValue::Map(m) => Ok(m),
        _ => Err(CsilCborError("csil cbor: expected map".to_string())),
    }
}

/// Build the canonical CBOR value tree for a IdentityRef.
fn csil_enc_identity_ref(csil_v: &IdentityRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    if let Some(csil_inner) = &csil_v.handle {
        csil_entries.push((cbor_text("handle"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.user_id {
        csil_entries.push((cbor_text("user_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a IdentityRef from a decoded CBOR value tree.
fn csil_dec_identity_ref(csil_root: &CsilCborValue) -> Result<IdentityRef, CsilCborError> {
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let user_id = match cbor_map_get(csil_root, "user_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let handle = match cbor_map_get(csil_root, "handle") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(IdentityRef {
        domain,
        user_id,
        handle,
    })
}

/// Encode a IdentityRef to canonical CSIL CBOR bytes.
pub fn encode_identity_ref(csil_v: &IdentityRef) -> Vec<u8> {
    cbor_encode(&csil_enc_identity_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a IdentityRef.
pub fn decode_identity_ref(csil_data: &[u8]) -> Result<IdentityRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_identity_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a User.
fn csil_enc_user(csil_v: &User) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(9);
    if let Some(csil_inner) = &csil_v.email {
        csil_entries.push((cbor_text("email"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    if let Some(csil_inner) = &csil_v.handle {
        csil_entries.push((cbor_text("handle"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("subject"), cbor_text(&csil_v.subject)));
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a User from a decoded CBOR value tree.
fn csil_dec_user(csil_root: &CsilCborValue) -> Result<User, CsilCborError> {
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let subject = {
        let csil_field = cbor_require(csil_root, "subject")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let handle = match cbor_map_get(csil_root, "handle") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let email = match cbor_map_get(csil_root, "email") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(User {
        user_id,
        domain,
        subject,
        handle,
        display_name,
        email,
        timezone,
        created_at,
        updated_at,
    })
}

/// Encode a User to canonical CSIL CBOR bytes.
pub fn encode_user(csil_v: &User) -> Vec<u8> {
    cbor_encode(&csil_enc_user(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a User.
pub fn decode_user(csil_data: &[u8]) -> Result<User, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_user(&csil_root)
}

/// Build the canonical CBOR value tree for a PersonSummary.
fn csil_enc_person_summary(csil_v: &PersonSummary) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    if let Some(csil_inner) = &csil_v.handle {
        csil_entries.push((cbor_text("handle"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("subject"), cbor_text(&csil_v.subject)));
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a PersonSummary from a decoded CBOR value tree.
fn csil_dec_person_summary(csil_root: &CsilCborValue) -> Result<PersonSummary, CsilCborError> {
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let subject = {
        let csil_field = cbor_require(csil_root, "subject")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let handle = match cbor_map_get(csil_root, "handle") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(PersonSummary {
        user_id,
        domain,
        subject,
        handle,
        display_name,
    })
}

/// Encode a PersonSummary to canonical CSIL CBOR bytes.
pub fn encode_person_summary(csil_v: &PersonSummary) -> Vec<u8> {
    cbor_encode(&csil_enc_person_summary(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a PersonSummary.
pub fn decode_person_summary(csil_data: &[u8]) -> Result<PersonSummary, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_person_summary(&csil_root)
}

/// Build the canonical CBOR value tree for a Calendar.
fn csil_enc_calendar(csil_v: &Calendar) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(16);
    csil_entries.push((cbor_text("kind"), csil_enc_calendar_kind(&csil_v.kind)));
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    if let Some(csil_inner) = &csil_v.color {
        csil_entries.push((cbor_text("color"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("owner"), csil_enc_person_summary(&csil_v.owner)));
    csil_entries.push((cbor_text("access"), csil_enc_access_role(&csil_v.access)));
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.feed_path {
        csil_entries.push((cbor_text("feed_path"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("is_default"), cbor_bool(csil_v.is_default)));
    if let Some(csil_inner) = &csil_v.source_url {
        csil_entries.push((cbor_text("source_url"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.sync_error {
        csil_entries.push((cbor_text("sync_error"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    csil_entries.push((
        cbor_text("access_path"),
        csil_enc_access_path(&csil_v.access_path),
    ));
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.last_synced_at {
        csil_entries.push((cbor_text("last_synced_at"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Calendar from a decoded CBOR value tree.
fn csil_dec_calendar(csil_root: &CsilCborValue) -> Result<Calendar, CsilCborError> {
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let owner = {
        let csil_field = cbor_require(csil_root, "owner")?;
        let csil_decode = csil_dec_person_summary;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let color = match cbor_map_get(csil_root, "color") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let kind = {
        let csil_field = cbor_require(csil_root, "kind")?;
        let csil_decode = csil_dec_calendar_kind;
        csil_decode(csil_field)?
    };
    let source_url = match cbor_map_get(csil_root, "source_url") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let last_synced_at = match cbor_map_get(csil_root, "last_synced_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let sync_error = match cbor_map_get(csil_root, "sync_error") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let is_default = {
        let csil_field = cbor_require(csil_root, "is_default")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    let access = {
        let csil_field = cbor_require(csil_root, "access")?;
        let csil_decode = csil_dec_access_role;
        csil_decode(csil_field)?
    };
    let access_path = {
        let csil_field = cbor_require(csil_root, "access_path")?;
        let csil_decode = csil_dec_access_path;
        csil_decode(csil_field)?
    };
    let feed_path = match cbor_map_get(csil_root, "feed_path") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Calendar {
        calendar_id,
        owner,
        name,
        description,
        color,
        timezone,
        kind,
        source_url,
        last_synced_at,
        sync_error,
        is_default,
        access,
        access_path,
        feed_path,
        created_at,
        updated_at,
    })
}

/// Encode a Calendar to canonical CSIL CBOR bytes.
pub fn encode_calendar(csil_v: &Calendar) -> Vec<u8> {
    cbor_encode(&csil_enc_calendar(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Calendar.
pub fn decode_calendar(csil_data: &[u8]) -> Result<Calendar, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_calendar(&csil_root)
}

/// Build the canonical CBOR value tree for a CreateCalendarRequest.
fn csil_enc_create_calendar_request(csil_v: &CreateCalendarRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    if let Some(csil_inner) = &csil_v.color {
        csil_entries.push((cbor_text("color"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a CreateCalendarRequest from a decoded CBOR value tree.
fn csil_dec_create_calendar_request(
    csil_root: &CsilCborValue,
) -> Result<CreateCalendarRequest, CsilCborError> {
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let color = match cbor_map_get(csil_root, "color") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(CreateCalendarRequest {
        name,
        description,
        color,
        timezone,
    })
}

/// Encode a CreateCalendarRequest to canonical CSIL CBOR bytes.
pub fn encode_create_calendar_request(csil_v: &CreateCalendarRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_create_calendar_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a CreateCalendarRequest.
pub fn decode_create_calendar_request(
    csil_data: &[u8],
) -> Result<CreateCalendarRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_create_calendar_request(&csil_root)
}

/// Build the canonical CBOR value tree for a UpdateCalendarRequest.
fn csil_enc_update_calendar_request(csil_v: &UpdateCalendarRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    if let Some(csil_inner) = &csil_v.name {
        csil_entries.push((cbor_text("name"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.color {
        csil_entries.push((cbor_text("color"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a UpdateCalendarRequest from a decoded CBOR value tree.
fn csil_dec_update_calendar_request(
    csil_root: &CsilCborValue,
) -> Result<UpdateCalendarRequest, CsilCborError> {
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = match cbor_map_get(csil_root, "name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let color = match cbor_map_get(csil_root, "color") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(UpdateCalendarRequest {
        calendar_id,
        name,
        description,
        color,
        timezone,
    })
}

/// Encode a UpdateCalendarRequest to canonical CSIL CBOR bytes.
pub fn encode_update_calendar_request(csil_v: &UpdateCalendarRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_update_calendar_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a UpdateCalendarRequest.
pub fn decode_update_calendar_request(
    csil_data: &[u8],
) -> Result<UpdateCalendarRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_update_calendar_request(&csil_root)
}

/// Build the canonical CBOR value tree for a SubscribeCalendarRequest.
fn csil_enc_subscribe_calendar_request(csil_v: &SubscribeCalendarRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    if let Some(csil_inner) = &csil_v.color {
        csil_entries.push((cbor_text("color"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("source_url"), cbor_text(&csil_v.source_url)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a SubscribeCalendarRequest from a decoded CBOR value tree.
fn csil_dec_subscribe_calendar_request(
    csil_root: &CsilCborValue,
) -> Result<SubscribeCalendarRequest, CsilCborError> {
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let source_url = {
        let csil_field = cbor_require(csil_root, "source_url")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let color = match cbor_map_get(csil_root, "color") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(SubscribeCalendarRequest {
        name,
        source_url,
        color,
    })
}

/// Encode a SubscribeCalendarRequest to canonical CSIL CBOR bytes.
pub fn encode_subscribe_calendar_request(csil_v: &SubscribeCalendarRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_subscribe_calendar_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a SubscribeCalendarRequest.
pub fn decode_subscribe_calendar_request(
    csil_data: &[u8],
) -> Result<SubscribeCalendarRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_subscribe_calendar_request(&csil_root)
}

/// Build the canonical CBOR value tree for a SetFeedRequest.
fn csil_enc_set_feed_request(csil_v: &SetFeedRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("enabled"), cbor_bool(csil_v.enabled)));
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a SetFeedRequest from a decoded CBOR value tree.
fn csil_dec_set_feed_request(csil_root: &CsilCborValue) -> Result<SetFeedRequest, CsilCborError> {
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let enabled = {
        let csil_field = cbor_require(csil_root, "enabled")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    Ok(SetFeedRequest {
        calendar_id,
        enabled,
    })
}

/// Encode a SetFeedRequest to canonical CSIL CBOR bytes.
pub fn encode_set_feed_request(csil_v: &SetFeedRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_set_feed_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a SetFeedRequest.
pub fn decode_set_feed_request(csil_data: &[u8]) -> Result<SetFeedRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_set_feed_request(&csil_root)
}

/// Build the canonical CBOR value tree for a CalendarList.
fn csil_enc_calendar_list(csil_v: &CalendarList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("calendars"),
        cbor_enc_array(&csil_v.calendars, csil_enc_calendar),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a CalendarList from a decoded CBOR value tree.
fn csil_dec_calendar_list(csil_root: &CsilCborValue) -> Result<CalendarList, CsilCborError> {
    let calendars = {
        let csil_field = cbor_require(csil_root, "calendars")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_calendar);
        csil_decode(csil_field)?
    };
    Ok(CalendarList { calendars })
}

/// Encode a CalendarList to canonical CSIL CBOR bytes.
pub fn encode_calendar_list(csil_v: &CalendarList) -> Vec<u8> {
    cbor_encode(&csil_enc_calendar_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a CalendarList.
pub fn decode_calendar_list(csil_data: &[u8]) -> Result<CalendarList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_calendar_list(&csil_root)
}

/// Build the canonical CBOR value tree for a Share.
fn csil_enc_share(csil_v: &Share) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("role"), csil_enc_share_role(&csil_v.role)));
    csil_entries.push((cbor_text("grantee"), csil_enc_identity_ref(&csil_v.grantee)));
    csil_entries.push((cbor_text("share_id"), cbor_text(&csil_v.share_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    if let Some(csil_inner) = &csil_v.grantee_person {
        csil_entries.push((
            cbor_text("grantee_person"),
            csil_enc_person_summary(csil_inner),
        ));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Share from a decoded CBOR value tree.
fn csil_dec_share(csil_root: &CsilCborValue) -> Result<Share, CsilCborError> {
    let share_id = {
        let csil_field = cbor_require(csil_root, "share_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee = {
        let csil_field = cbor_require(csil_root, "grantee")?;
        let csil_decode = csil_dec_identity_ref;
        csil_decode(csil_field)?
    };
    let role = {
        let csil_field = cbor_require(csil_root, "role")?;
        let csil_decode = csil_dec_share_role;
        csil_decode(csil_field)?
    };
    let grantee_person = match cbor_map_get(csil_root, "grantee_person") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Share {
        share_id,
        calendar_id,
        grantee,
        role,
        grantee_person,
        created_at,
    })
}

/// Encode a Share to canonical CSIL CBOR bytes.
pub fn encode_share(csil_v: &Share) -> Vec<u8> {
    cbor_encode(&csil_enc_share(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Share.
pub fn decode_share(csil_data: &[u8]) -> Result<Share, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_share(&csil_root)
}

/// Build the canonical CBOR value tree for a ShareList.
fn csil_enc_share_list(csil_v: &ShareList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("shares"),
        cbor_enc_array(&csil_v.shares, csil_enc_share),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ShareList from a decoded CBOR value tree.
fn csil_dec_share_list(csil_root: &CsilCborValue) -> Result<ShareList, CsilCborError> {
    let shares = {
        let csil_field = cbor_require(csil_root, "shares")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_share);
        csil_decode(csil_field)?
    };
    Ok(ShareList { shares })
}

/// Encode a ShareList to canonical CSIL CBOR bytes.
pub fn encode_share_list(csil_v: &ShareList) -> Vec<u8> {
    cbor_encode(&csil_enc_share_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ShareList.
pub fn decode_share_list(csil_data: &[u8]) -> Result<ShareList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_share_list(&csil_root)
}

/// Build the canonical CBOR value tree for a Delegation.
fn csil_enc_delegation(csil_v: &Delegation) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(10);
    if let Some(csil_inner) = &csil_v.note {
        csil_entries.push((cbor_text("note"), cbor_text(csil_inner)));
    }
    csil_entries.push((
        cbor_text("scopes"),
        cbor_enc_array(&csil_v.scopes, csil_enc_scope),
    ));
    csil_entries.push((
        cbor_text("delegate"),
        csil_enc_identity_ref(&csil_v.delegate),
    ));
    csil_entries.push((
        cbor_text("principal"),
        csil_enc_person_summary(&csil_v.principal),
    ));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.expires_at {
        csil_entries.push((cbor_text("expires_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.revoked_at {
        csil_entries.push((cbor_text("revoked_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.calendar_ids {
        csil_entries.push((
            cbor_text("calendar_ids"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    csil_entries.push((cbor_text("delegation_id"), cbor_text(&csil_v.delegation_id)));
    if let Some(csil_inner) = &csil_v.delegate_person {
        csil_entries.push((
            cbor_text("delegate_person"),
            csil_enc_person_summary(csil_inner),
        ));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Delegation from a decoded CBOR value tree.
fn csil_dec_delegation(csil_root: &CsilCborValue) -> Result<Delegation, CsilCborError> {
    let delegation_id = {
        let csil_field = cbor_require(csil_root, "delegation_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let principal = {
        let csil_field = cbor_require(csil_root, "principal")?;
        let csil_decode = csil_dec_person_summary;
        csil_decode(csil_field)?
    };
    let delegate = {
        let csil_field = cbor_require(csil_root, "delegate")?;
        let csil_decode = csil_dec_identity_ref;
        csil_decode(csil_field)?
    };
    let delegate_person = match cbor_map_get(csil_root, "delegate_person") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let scopes = {
        let csil_field = cbor_require(csil_root, "scopes")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_scope);
        csil_decode(csil_field)?
    };
    let calendar_ids = match cbor_map_get(csil_root, "calendar_ids") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let note = match cbor_map_get(csil_root, "note") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let expires_at = match cbor_map_get(csil_root, "expires_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let revoked_at = match cbor_map_get(csil_root, "revoked_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Delegation {
        delegation_id,
        principal,
        delegate,
        delegate_person,
        scopes,
        calendar_ids,
        note,
        expires_at,
        created_at,
        revoked_at,
    })
}

/// Encode a Delegation to canonical CSIL CBOR bytes.
pub fn encode_delegation(csil_v: &Delegation) -> Vec<u8> {
    cbor_encode(&csil_enc_delegation(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Delegation.
pub fn decode_delegation(csil_data: &[u8]) -> Result<Delegation, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_delegation(&csil_root)
}

/// Build the canonical CBOR value tree for a DelegationList.
fn csil_enc_delegation_list(csil_v: &DelegationList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("delegations"),
        cbor_enc_array(&csil_v.delegations, csil_enc_delegation),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DelegationList from a decoded CBOR value tree.
fn csil_dec_delegation_list(csil_root: &CsilCborValue) -> Result<DelegationList, CsilCborError> {
    let delegations = {
        let csil_field = cbor_require(csil_root, "delegations")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_delegation);
        csil_decode(csil_field)?
    };
    Ok(DelegationList { delegations })
}

/// Encode a DelegationList to canonical CSIL CBOR bytes.
pub fn encode_delegation_list(csil_v: &DelegationList) -> Vec<u8> {
    cbor_encode(&csil_enc_delegation_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DelegationList.
pub fn decode_delegation_list(csil_data: &[u8]) -> Result<DelegationList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_delegation_list(&csil_root)
}

/// Build the canonical CBOR value tree for a AssumePrincipalRequest.
fn csil_enc_assume_principal_request(csil_v: &AssumePrincipalRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("principal_user_id"),
        cbor_text(&csil_v.principal_user_id),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AssumePrincipalRequest from a decoded CBOR value tree.
fn csil_dec_assume_principal_request(
    csil_root: &CsilCborValue,
) -> Result<AssumePrincipalRequest, CsilCborError> {
    let principal_user_id = {
        let csil_field = cbor_require(csil_root, "principal_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(AssumePrincipalRequest { principal_user_id })
}

/// Encode a AssumePrincipalRequest to canonical CSIL CBOR bytes.
pub fn encode_assume_principal_request(csil_v: &AssumePrincipalRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_assume_principal_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AssumePrincipalRequest.
pub fn decode_assume_principal_request(
    csil_data: &[u8],
) -> Result<AssumePrincipalRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_assume_principal_request(&csil_root)
}

/// Build the canonical CBOR value tree for a Recurrence.
fn csil_enc_recurrence(csil_v: &Recurrence) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("rrule"), cbor_text(&csil_v.rrule)));
    if let Some(csil_inner) = &csil_v.rdates {
        csil_entries.push((
            cbor_text("rdates"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    if let Some(csil_inner) = &csil_v.exdates {
        csil_entries.push((
            cbor_text("exdates"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Recurrence from a decoded CBOR value tree.
fn csil_dec_recurrence(csil_root: &CsilCborValue) -> Result<Recurrence, CsilCborError> {
    let rrule = {
        let csil_field = cbor_require(csil_root, "rrule")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let exdates = match cbor_map_get(csil_root, "exdates") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let rdates = match cbor_map_get(csil_root, "rdates") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Recurrence {
        rrule,
        exdates,
        rdates,
    })
}

/// Encode a Recurrence to canonical CSIL CBOR bytes.
pub fn encode_recurrence(csil_v: &Recurrence) -> Vec<u8> {
    cbor_encode(&csil_enc_recurrence(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Recurrence.
pub fn decode_recurrence(csil_data: &[u8]) -> Result<Recurrence, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_recurrence(&csil_root)
}

/// Build the canonical CBOR value tree for a Attendee.
fn csil_enc_attendee(csil_v: &Attendee) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(8);
    if let Some(csil_inner) = &csil_v.name {
        csil_entries.push((cbor_text("name"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.role {
        csil_entries.push((cbor_text("role"), csil_enc_attendee_role(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.rsvp {
        csil_entries.push((cbor_text("rsvp"), cbor_bool(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.email {
        csil_entries.push((cbor_text("email"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.person {
        csil_entries.push((cbor_text("person"), csil_enc_person_summary(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.status {
        csil_entries.push((
            cbor_text("status"),
            csil_enc_participation_status(csil_inner),
        ));
    }
    if let Some(csil_inner) = &csil_v.identity {
        csil_entries.push((cbor_text("identity"), csil_enc_identity_ref(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.responded_by {
        csil_entries.push((
            cbor_text("responded_by"),
            csil_enc_person_summary(csil_inner),
        ));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Attendee from a decoded CBOR value tree.
fn csil_dec_attendee(csil_root: &CsilCborValue) -> Result<Attendee, CsilCborError> {
    let identity = match cbor_map_get(csil_root, "identity") {
        Some(csil_field) => {
            let csil_decode = csil_dec_identity_ref;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let email = match cbor_map_get(csil_root, "email") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let name = match cbor_map_get(csil_root, "name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let role = match cbor_map_get(csil_root, "role") {
        Some(csil_field) => {
            let csil_decode = csil_dec_attendee_role;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let status = match cbor_map_get(csil_root, "status") {
        Some(csil_field) => {
            let csil_decode = csil_dec_participation_status;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let rsvp = match cbor_map_get(csil_root, "rsvp") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let person = match cbor_map_get(csil_root, "person") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let responded_by = match cbor_map_get(csil_root, "responded_by") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Attendee {
        identity,
        email,
        name,
        role,
        status,
        rsvp,
        person,
        responded_by,
    })
}

/// Encode a Attendee to canonical CSIL CBOR bytes.
pub fn encode_attendee(csil_v: &Attendee) -> Vec<u8> {
    cbor_encode(&csil_enc_attendee(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Attendee.
pub fn decode_attendee(csil_data: &[u8]) -> Result<Attendee, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_attendee(&csil_root)
}

/// Build the canonical CBOR value tree for a Reminder.
fn csil_enc_reminder(csil_v: &Reminder) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.method {
        csil_entries.push((cbor_text("method"), csil_enc_reminder_method(csil_inner)));
    }
    csil_entries.push((cbor_text("minutes_before"), cbor_int(csil_v.minutes_before)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Reminder from a decoded CBOR value tree.
fn csil_dec_reminder(csil_root: &CsilCborValue) -> Result<Reminder, CsilCborError> {
    let minutes_before = {
        let csil_field = cbor_require(csil_root, "minutes_before")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    let method = match cbor_map_get(csil_root, "method") {
        Some(csil_field) => {
            let csil_decode = csil_dec_reminder_method;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Reminder {
        minutes_before,
        method,
    })
}

/// Encode a Reminder to canonical CSIL CBOR bytes.
pub fn encode_reminder(csil_v: &Reminder) -> Vec<u8> {
    cbor_encode(&csil_enc_reminder(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Reminder.
pub fn decode_reminder(csil_data: &[u8]) -> Result<Reminder, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_reminder(&csil_root)
}

/// Build the canonical CBOR value tree for a Event.
fn csil_enc_event(csil_v: &Event) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(31);
    if let Some(csil_inner) = &csil_v.end {
        csil_entries.push((cbor_text("end"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.uid {
        csil_entries.push((cbor_text("uid"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.url {
        csil_entries.push((cbor_text("url"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.color {
        csil_entries.push((cbor_text("color"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.start {
        csil_entries.push((cbor_text("start"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("title"), cbor_text(&csil_v.title)));
    if let Some(csil_inner) = &csil_v.status {
        csil_entries.push((cbor_text("status"), csil_enc_event_status(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.all_day {
        csil_entries.push((cbor_text("all_day"), cbor_bool(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.end_date {
        csil_entries.push((cbor_text("end_date"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    if let Some(csil_inner) = &csil_v.location {
        csil_entries.push((cbor_text("location"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("sequence"), cbor_int(csil_v.sequence)));
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.attendees {
        csil_entries.push((
            cbor_text("attendees"),
            cbor_enc_array(csil_inner, csil_enc_attendee),
        ));
    }
    if let Some(csil_inner) = &csil_v.organizer {
        csil_entries.push((cbor_text("organizer"), csil_enc_person_summary(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.reminders {
        csil_entries.push((
            cbor_text("reminders"),
            cbor_enc_array(csil_inner, csil_enc_reminder),
        ));
    }
    if let Some(csil_inner) = &csil_v.categories {
        csil_entries.push((
            cbor_text("categories"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.created_by {
        csil_entries.push((cbor_text("created_by"), csil_enc_person_summary(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence {
        csil_entries.push((cbor_text("recurrence"), csil_enc_recurrence(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.start_date {
        csil_entries.push((cbor_text("start_date"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.updated_by {
        csil_entries.push((cbor_text("updated_by"), csil_enc_person_summary(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.visibility {
        csil_entries.push((
            cbor_text("visibility"),
            csil_enc_event_visibility(csil_inner),
        ));
    }
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.updated_via {
        csil_entries.push((cbor_text("updated_via"), csil_enc_acting_via(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.transparency {
        csil_entries.push((
            cbor_text("transparency"),
            csil_enc_event_transparency(csil_inner),
        ));
    }
    if let Some(csil_inner) = &csil_v.recurrence_id {
        csil_entries.push((cbor_text("recurrence_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.conference_url {
        csil_entries.push((cbor_text("conference_url"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.master_event_id {
        csil_entries.push((cbor_text("master_event_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Event from a decoded CBOR value tree.
fn csil_dec_event(csil_root: &CsilCborValue) -> Result<Event, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let uid = match cbor_map_get(csil_root, "uid") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let title = {
        let csil_field = cbor_require(csil_root, "title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let location = match cbor_map_get(csil_root, "location") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let url = match cbor_map_get(csil_root, "url") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let conference_url = match cbor_map_get(csil_root, "conference_url") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let start = match cbor_map_get(csil_root, "start") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let end = match cbor_map_get(csil_root, "end") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let all_day = match cbor_map_get(csil_root, "all_day") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let start_date = match cbor_map_get(csil_root, "start_date") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let end_date = match cbor_map_get(csil_root, "end_date") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let status = match cbor_map_get(csil_root, "status") {
        Some(csil_field) => {
            let csil_decode = csil_dec_event_status;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let transparency = match cbor_map_get(csil_root, "transparency") {
        Some(csil_field) => {
            let csil_decode = csil_dec_event_transparency;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let visibility = match cbor_map_get(csil_root, "visibility") {
        Some(csil_field) => {
            let csil_decode = csil_dec_event_visibility;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence = match cbor_map_get(csil_root, "recurrence") {
        Some(csil_field) => {
            let csil_decode = csil_dec_recurrence;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let master_event_id = match cbor_map_get(csil_root, "master_event_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_id = match cbor_map_get(csil_root, "recurrence_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let attendees = match cbor_map_get(csil_root, "attendees") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_attendee);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let reminders = match cbor_map_get(csil_root, "reminders") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_reminder);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let categories = match cbor_map_get(csil_root, "categories") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let color = match cbor_map_get(csil_root, "color") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let organizer = match cbor_map_get(csil_root, "organizer") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_by = match cbor_map_get(csil_root, "created_by") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let updated_by = match cbor_map_get(csil_root, "updated_by") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let updated_via = match cbor_map_get(csil_root, "updated_via") {
        Some(csil_field) => {
            let csil_decode = csil_dec_acting_via;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let sequence = {
        let csil_field = cbor_require(csil_root, "sequence")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Event {
        event_id,
        calendar_id,
        uid,
        title,
        description,
        location,
        url,
        conference_url,
        start,
        end,
        all_day,
        start_date,
        end_date,
        timezone,
        status,
        transparency,
        visibility,
        recurrence,
        master_event_id,
        recurrence_id,
        attendees,
        reminders,
        categories,
        color,
        organizer,
        created_by,
        updated_by,
        updated_via,
        sequence,
        created_at,
        updated_at,
    })
}

/// Encode a Event to canonical CSIL CBOR bytes.
pub fn encode_event(csil_v: &Event) -> Vec<u8> {
    cbor_encode(&csil_enc_event(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Event.
pub fn decode_event(csil_data: &[u8]) -> Result<Event, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_event(&csil_root)
}

/// Build the canonical CBOR value tree for a Occurrence.
fn csil_enc_occurrence(csil_v: &Occurrence) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(7);
    csil_entries.push((cbor_text("end"), cbor_text(&csil_v.end)));
    csil_entries.push((cbor_text("event"), csil_enc_event(&csil_v.event)));
    csil_entries.push((cbor_text("start"), cbor_text(&csil_v.start)));
    csil_entries.push((cbor_text("access"), csil_enc_access_role(&csil_v.access)));
    csil_entries.push((cbor_text("invited"), cbor_bool(csil_v.invited)));
    csil_entries.push((cbor_text("busy_only"), cbor_bool(csil_v.busy_only)));
    if let Some(csil_inner) = &csil_v.recurrence_id {
        csil_entries.push((cbor_text("recurrence_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Occurrence from a decoded CBOR value tree.
fn csil_dec_occurrence(csil_root: &CsilCborValue) -> Result<Occurrence, CsilCborError> {
    let event = {
        let csil_field = cbor_require(csil_root, "event")?;
        let csil_decode = csil_dec_event;
        csil_decode(csil_field)?
    };
    let start = {
        let csil_field = cbor_require(csil_root, "start")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let end = {
        let csil_field = cbor_require(csil_root, "end")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let recurrence_id = match cbor_map_get(csil_root, "recurrence_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let access = {
        let csil_field = cbor_require(csil_root, "access")?;
        let csil_decode = csil_dec_access_role;
        csil_decode(csil_field)?
    };
    let busy_only = {
        let csil_field = cbor_require(csil_root, "busy_only")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    let invited = {
        let csil_field = cbor_require(csil_root, "invited")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    Ok(Occurrence {
        event,
        start,
        end,
        recurrence_id,
        access,
        busy_only,
        invited,
    })
}

/// Encode a Occurrence to canonical CSIL CBOR bytes.
pub fn encode_occurrence(csil_v: &Occurrence) -> Vec<u8> {
    cbor_encode(&csil_enc_occurrence(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Occurrence.
pub fn decode_occurrence(csil_data: &[u8]) -> Result<Occurrence, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_occurrence(&csil_root)
}

/// Build the canonical CBOR value tree for a ListEventsRequest.
fn csil_enc_list_events_request(csil_v: &ListEventsRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("range_end"), cbor_text(&csil_v.range_end)));
    csil_entries.push((cbor_text("range_start"), cbor_text(&csil_v.range_start)));
    if let Some(csil_inner) = &csil_v.calendar_ids {
        csil_entries.push((
            cbor_text("calendar_ids"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    if let Some(csil_inner) = &csil_v.include_invitations {
        csil_entries.push((cbor_text("include_invitations"), cbor_bool(*csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ListEventsRequest from a decoded CBOR value tree.
fn csil_dec_list_events_request(
    csil_root: &CsilCborValue,
) -> Result<ListEventsRequest, CsilCborError> {
    let range_start = {
        let csil_field = cbor_require(csil_root, "range_start")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let range_end = {
        let csil_field = cbor_require(csil_root, "range_end")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let calendar_ids = match cbor_map_get(csil_root, "calendar_ids") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let include_invitations = match cbor_map_get(csil_root, "include_invitations") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(ListEventsRequest {
        range_start,
        range_end,
        calendar_ids,
        include_invitations,
    })
}

/// Encode a ListEventsRequest to canonical CSIL CBOR bytes.
pub fn encode_list_events_request(csil_v: &ListEventsRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_list_events_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ListEventsRequest.
pub fn decode_list_events_request(csil_data: &[u8]) -> Result<ListEventsRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_list_events_request(&csil_root)
}

/// Build the canonical CBOR value tree for a OccurrenceList.
fn csil_enc_occurrence_list(csil_v: &OccurrenceList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("truncated"), cbor_bool(csil_v.truncated)));
    csil_entries.push((
        cbor_text("occurrences"),
        cbor_enc_array(&csil_v.occurrences, csil_enc_occurrence),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a OccurrenceList from a decoded CBOR value tree.
fn csil_dec_occurrence_list(csil_root: &CsilCborValue) -> Result<OccurrenceList, CsilCborError> {
    let occurrences = {
        let csil_field = cbor_require(csil_root, "occurrences")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_occurrence);
        csil_decode(csil_field)?
    };
    let truncated = {
        let csil_field = cbor_require(csil_root, "truncated")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    Ok(OccurrenceList {
        occurrences,
        truncated,
    })
}

/// Encode a OccurrenceList to canonical CSIL CBOR bytes.
pub fn encode_occurrence_list(csil_v: &OccurrenceList) -> Vec<u8> {
    cbor_encode(&csil_enc_occurrence_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a OccurrenceList.
pub fn decode_occurrence_list(csil_data: &[u8]) -> Result<OccurrenceList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_occurrence_list(&csil_root)
}

/// Build the canonical CBOR value tree for a UpdateEventRequest.
fn csil_enc_update_event_request(csil_v: &UpdateEventRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("event"), csil_enc_event(&csil_v.event)));
    if let Some(csil_inner) = &csil_v.scope {
        csil_entries.push((cbor_text("scope"), csil_enc_recurrence_scope(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_id {
        csil_entries.push((cbor_text("recurrence_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a UpdateEventRequest from a decoded CBOR value tree.
fn csil_dec_update_event_request(
    csil_root: &CsilCborValue,
) -> Result<UpdateEventRequest, CsilCborError> {
    let event = {
        let csil_field = cbor_require(csil_root, "event")?;
        let csil_decode = csil_dec_event;
        csil_decode(csil_field)?
    };
    let scope = match cbor_map_get(csil_root, "scope") {
        Some(csil_field) => {
            let csil_decode = csil_dec_recurrence_scope;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_id = match cbor_map_get(csil_root, "recurrence_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(UpdateEventRequest {
        event,
        scope,
        recurrence_id,
    })
}

/// Encode a UpdateEventRequest to canonical CSIL CBOR bytes.
pub fn encode_update_event_request(csil_v: &UpdateEventRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_update_event_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a UpdateEventRequest.
pub fn decode_update_event_request(csil_data: &[u8]) -> Result<UpdateEventRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_update_event_request(&csil_root)
}

/// Build the canonical CBOR value tree for a DeleteEventRequest.
fn csil_enc_delete_event_request(csil_v: &DeleteEventRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    if let Some(csil_inner) = &csil_v.scope {
        csil_entries.push((cbor_text("scope"), csil_enc_recurrence_scope(csil_inner)));
    }
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    if let Some(csil_inner) = &csil_v.recurrence_id {
        csil_entries.push((cbor_text("recurrence_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DeleteEventRequest from a decoded CBOR value tree.
fn csil_dec_delete_event_request(
    csil_root: &CsilCborValue,
) -> Result<DeleteEventRequest, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let scope = match cbor_map_get(csil_root, "scope") {
        Some(csil_field) => {
            let csil_decode = csil_dec_recurrence_scope;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_id = match cbor_map_get(csil_root, "recurrence_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(DeleteEventRequest {
        event_id,
        scope,
        recurrence_id,
    })
}

/// Encode a DeleteEventRequest to canonical CSIL CBOR bytes.
pub fn encode_delete_event_request(csil_v: &DeleteEventRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_delete_event_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DeleteEventRequest.
pub fn decode_delete_event_request(csil_data: &[u8]) -> Result<DeleteEventRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_delete_event_request(&csil_root)
}

/// Build the canonical CBOR value tree for a MoveEventRequest.
fn csil_enc_move_event_request(csil_v: &MoveEventRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MoveEventRequest from a decoded CBOR value tree.
fn csil_dec_move_event_request(
    csil_root: &CsilCborValue,
) -> Result<MoveEventRequest, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MoveEventRequest {
        event_id,
        calendar_id,
    })
}

/// Encode a MoveEventRequest to canonical CSIL CBOR bytes.
pub fn encode_move_event_request(csil_v: &MoveEventRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_move_event_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MoveEventRequest.
pub fn decode_move_event_request(csil_data: &[u8]) -> Result<MoveEventRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_move_event_request(&csil_root)
}

/// Build the canonical CBOR value tree for a RespondRequest.
fn csil_enc_respond_request(csil_v: &RespondRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((
        cbor_text("status"),
        csil_enc_participation_status(&csil_v.status),
    ));
    if let Some(csil_inner) = &csil_v.comment {
        csil_entries.push((cbor_text("comment"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    if let Some(csil_inner) = &csil_v.recurrence_id {
        csil_entries.push((cbor_text("recurrence_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a RespondRequest from a decoded CBOR value tree.
fn csil_dec_respond_request(csil_root: &CsilCborValue) -> Result<RespondRequest, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let status = {
        let csil_field = cbor_require(csil_root, "status")?;
        let csil_decode = csil_dec_participation_status;
        csil_decode(csil_field)?
    };
    let recurrence_id = match cbor_map_get(csil_root, "recurrence_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let comment = match cbor_map_get(csil_root, "comment") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(RespondRequest {
        event_id,
        status,
        recurrence_id,
        comment,
    })
}

/// Encode a RespondRequest to canonical CSIL CBOR bytes.
pub fn encode_respond_request(csil_v: &RespondRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_respond_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a RespondRequest.
pub fn decode_respond_request(csil_data: &[u8]) -> Result<RespondRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_respond_request(&csil_root)
}

/// Build the canonical CBOR value tree for a SearchEventsRequest.
fn csil_enc_search_events_request(csil_v: &SearchEventsRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("query"), cbor_text(&csil_v.query)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a SearchEventsRequest from a decoded CBOR value tree.
fn csil_dec_search_events_request(
    csil_root: &CsilCborValue,
) -> Result<SearchEventsRequest, CsilCborError> {
    let query = {
        let csil_field = cbor_require(csil_root, "query")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(SearchEventsRequest { query, limit })
}

/// Encode a SearchEventsRequest to canonical CSIL CBOR bytes.
pub fn encode_search_events_request(csil_v: &SearchEventsRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_search_events_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a SearchEventsRequest.
pub fn decode_search_events_request(
    csil_data: &[u8],
) -> Result<SearchEventsRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_search_events_request(&csil_root)
}

/// Build the canonical CBOR value tree for a EventList.
fn csil_enc_event_list(csil_v: &EventList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("events"),
        cbor_enc_array(&csil_v.events, csil_enc_event),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a EventList from a decoded CBOR value tree.
fn csil_dec_event_list(csil_root: &CsilCborValue) -> Result<EventList, CsilCborError> {
    let events = {
        let csil_field = cbor_require(csil_root, "events")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_event);
        csil_decode(csil_field)?
    };
    Ok(EventList { events })
}

/// Encode a EventList to canonical CSIL CBOR bytes.
pub fn encode_event_list(csil_v: &EventList) -> Vec<u8> {
    cbor_encode(&csil_enc_event_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EventList.
pub fn decode_event_list(csil_data: &[u8]) -> Result<EventList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_event_list(&csil_root)
}

/// Build the canonical CBOR value tree for a BusyInterval.
fn csil_enc_busy_interval(csil_v: &BusyInterval) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("end"), cbor_text(&csil_v.end)));
    csil_entries.push((cbor_text("start"), cbor_text(&csil_v.start)));
    if let Some(csil_inner) = &csil_v.tentative {
        csil_entries.push((cbor_text("tentative"), cbor_bool(*csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a BusyInterval from a decoded CBOR value tree.
fn csil_dec_busy_interval(csil_root: &CsilCborValue) -> Result<BusyInterval, CsilCborError> {
    let start = {
        let csil_field = cbor_require(csil_root, "start")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let end = {
        let csil_field = cbor_require(csil_root, "end")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let tentative = match cbor_map_get(csil_root, "tentative") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(BusyInterval {
        start,
        end,
        tentative,
    })
}

/// Encode a BusyInterval to canonical CSIL CBOR bytes.
pub fn encode_busy_interval(csil_v: &BusyInterval) -> Vec<u8> {
    cbor_encode(&csil_enc_busy_interval(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a BusyInterval.
pub fn decode_busy_interval(csil_data: &[u8]) -> Result<BusyInterval, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_busy_interval(&csil_root)
}

/// Build the canonical CBOR value tree for a FreeBusyRequest.
fn csil_enc_free_busy_request(csil_v: &FreeBusyRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((
        cbor_text("people"),
        cbor_enc_array(&csil_v.people, csil_enc_identity_ref),
    ));
    csil_entries.push((cbor_text("range_end"), cbor_text(&csil_v.range_end)));
    csil_entries.push((cbor_text("range_start"), cbor_text(&csil_v.range_start)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a FreeBusyRequest from a decoded CBOR value tree.
fn csil_dec_free_busy_request(csil_root: &CsilCborValue) -> Result<FreeBusyRequest, CsilCborError> {
    let people = {
        let csil_field = cbor_require(csil_root, "people")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_identity_ref);
        csil_decode(csil_field)?
    };
    let range_start = {
        let csil_field = cbor_require(csil_root, "range_start")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let range_end = {
        let csil_field = cbor_require(csil_root, "range_end")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(FreeBusyRequest {
        people,
        range_start,
        range_end,
    })
}

/// Encode a FreeBusyRequest to canonical CSIL CBOR bytes.
pub fn encode_free_busy_request(csil_v: &FreeBusyRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_free_busy_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a FreeBusyRequest.
pub fn decode_free_busy_request(csil_data: &[u8]) -> Result<FreeBusyRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_free_busy_request(&csil_root)
}

/// Build the canonical CBOR value tree for a PersonFreeBusy.
fn csil_enc_person_free_busy(csil_v: &PersonFreeBusy) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((
        cbor_text("busy"),
        cbor_enc_array(&csil_v.busy, csil_enc_busy_interval),
    ));
    if let Some(csil_inner) = &csil_v.person {
        csil_entries.push((cbor_text("person"), csil_enc_person_summary(csil_inner)));
    }
    csil_entries.push((cbor_text("visible"), cbor_bool(csil_v.visible)));
    csil_entries.push((
        cbor_text("identity"),
        csil_enc_identity_ref(&csil_v.identity),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a PersonFreeBusy from a decoded CBOR value tree.
fn csil_dec_person_free_busy(csil_root: &CsilCborValue) -> Result<PersonFreeBusy, CsilCborError> {
    let identity = {
        let csil_field = cbor_require(csil_root, "identity")?;
        let csil_decode = csil_dec_identity_ref;
        csil_decode(csil_field)?
    };
    let person = match cbor_map_get(csil_root, "person") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let busy = {
        let csil_field = cbor_require(csil_root, "busy")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_busy_interval);
        csil_decode(csil_field)?
    };
    let visible = {
        let csil_field = cbor_require(csil_root, "visible")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    Ok(PersonFreeBusy {
        identity,
        person,
        busy,
        visible,
    })
}

/// Encode a PersonFreeBusy to canonical CSIL CBOR bytes.
pub fn encode_person_free_busy(csil_v: &PersonFreeBusy) -> Vec<u8> {
    cbor_encode(&csil_enc_person_free_busy(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a PersonFreeBusy.
pub fn decode_person_free_busy(csil_data: &[u8]) -> Result<PersonFreeBusy, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_person_free_busy(&csil_root)
}

/// Build the canonical CBOR value tree for a FreeBusyResponse.
fn csil_enc_free_busy_response(csil_v: &FreeBusyResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("people"),
        cbor_enc_array(&csil_v.people, csil_enc_person_free_busy),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a FreeBusyResponse from a decoded CBOR value tree.
fn csil_dec_free_busy_response(
    csil_root: &CsilCborValue,
) -> Result<FreeBusyResponse, CsilCborError> {
    let people = {
        let csil_field = cbor_require(csil_root, "people")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_person_free_busy);
        csil_decode(csil_field)?
    };
    Ok(FreeBusyResponse { people })
}

/// Encode a FreeBusyResponse to canonical CSIL CBOR bytes.
pub fn encode_free_busy_response(csil_v: &FreeBusyResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_free_busy_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a FreeBusyResponse.
pub fn decode_free_busy_response(csil_data: &[u8]) -> Result<FreeBusyResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_free_busy_response(&csil_root)
}

/// Build the canonical CBOR value tree for a ImportIcsRequest.
fn csil_enc_import_ics_request(csil_v: &ImportIcsRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("ics"), cbor_text(&csil_v.ics)));
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ImportIcsRequest from a decoded CBOR value tree.
fn csil_dec_import_ics_request(
    csil_root: &CsilCborValue,
) -> Result<ImportIcsRequest, CsilCborError> {
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let ics = {
        let csil_field = cbor_require(csil_root, "ics")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ImportIcsRequest { calendar_id, ics })
}

/// Encode a ImportIcsRequest to canonical CSIL CBOR bytes.
pub fn encode_import_ics_request(csil_v: &ImportIcsRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_import_ics_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ImportIcsRequest.
pub fn decode_import_ics_request(csil_data: &[u8]) -> Result<ImportIcsRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_import_ics_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ImportResult.
fn csil_enc_import_result(csil_v: &ImportResult) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("created"), cbor_uint(csil_v.created)));
    csil_entries.push((cbor_text("skipped"), cbor_uint(csil_v.skipped)));
    csil_entries.push((cbor_text("updated"), cbor_uint(csil_v.updated)));
    if let Some(csil_inner) = &csil_v.warnings {
        csil_entries.push((
            cbor_text("warnings"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ImportResult from a decoded CBOR value tree.
fn csil_dec_import_result(csil_root: &CsilCborValue) -> Result<ImportResult, CsilCborError> {
    let created = {
        let csil_field = cbor_require(csil_root, "created")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    let updated = {
        let csil_field = cbor_require(csil_root, "updated")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    let skipped = {
        let csil_field = cbor_require(csil_root, "skipped")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    let warnings = match cbor_map_get(csil_root, "warnings") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(ImportResult {
        created,
        updated,
        skipped,
        warnings,
    })
}

/// Encode a ImportResult to canonical CSIL CBOR bytes.
pub fn encode_import_result(csil_v: &ImportResult) -> Vec<u8> {
    cbor_encode(&csil_enc_import_result(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ImportResult.
pub fn decode_import_result(csil_data: &[u8]) -> Result<ImportResult, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_import_result(&csil_root)
}

/// Build the canonical CBOR value tree for a ExportCalendarRequest.
fn csil_enc_export_calendar_request(csil_v: &ExportCalendarRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("calendar_id"), cbor_text(&csil_v.calendar_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ExportCalendarRequest from a decoded CBOR value tree.
fn csil_dec_export_calendar_request(
    csil_root: &CsilCborValue,
) -> Result<ExportCalendarRequest, CsilCborError> {
    let calendar_id = {
        let csil_field = cbor_require(csil_root, "calendar_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ExportCalendarRequest { calendar_id })
}

/// Encode a ExportCalendarRequest to canonical CSIL CBOR bytes.
pub fn encode_export_calendar_request(csil_v: &ExportCalendarRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_export_calendar_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ExportCalendarRequest.
pub fn decode_export_calendar_request(
    csil_data: &[u8],
) -> Result<ExportCalendarRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_export_calendar_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ExportEventRequest.
fn csil_enc_export_event_request(csil_v: &ExportEventRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.method {
        csil_entries.push((cbor_text("method"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ExportEventRequest from a decoded CBOR value tree.
fn csil_dec_export_event_request(
    csil_root: &CsilCborValue,
) -> Result<ExportEventRequest, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let method = match cbor_map_get(csil_root, "method") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(ExportEventRequest { event_id, method })
}

/// Encode a ExportEventRequest to canonical CSIL CBOR bytes.
pub fn encode_export_event_request(csil_v: &ExportEventRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_export_event_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ExportEventRequest.
pub fn decode_export_event_request(csil_data: &[u8]) -> Result<ExportEventRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_export_event_request(&csil_root)
}

/// Build the canonical CBOR value tree for a IcsDocument.
fn csil_enc_ics_document(csil_v: &IcsDocument) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("ics"), cbor_text(&csil_v.ics)));
    csil_entries.push((cbor_text("filename"), cbor_text(&csil_v.filename)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a IcsDocument from a decoded CBOR value tree.
fn csil_dec_ics_document(csil_root: &CsilCborValue) -> Result<IcsDocument, CsilCborError> {
    let ics = {
        let csil_field = cbor_require(csil_root, "ics")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let filename = {
        let csil_field = cbor_require(csil_root, "filename")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(IcsDocument { ics, filename })
}

/// Encode a IcsDocument to canonical CSIL CBOR bytes.
pub fn encode_ics_document(csil_v: &IcsDocument) -> Vec<u8> {
    cbor_encode(&csil_enc_ics_document(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a IcsDocument.
pub fn decode_ics_document(csil_data: &[u8]) -> Result<IcsDocument, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_ics_document(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditEntry.
fn csil_enc_audit_entry(csil_v: &AuditEntry) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(9);
    csil_entries.push((cbor_text("at"), cbor_text(&csil_v.at)));
    csil_entries.push((cbor_text("via"), csil_enc_acting_via(&csil_v.via)));
    csil_entries.push((cbor_text("actor"), csil_enc_person_summary(&csil_v.actor)));
    if let Some(csil_inner) = &csil_v.target {
        csil_entries.push((cbor_text("target"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("outcome"), cbor_text(&csil_v.outcome)));
    csil_entries.push((cbor_text("audit_id"), cbor_text(&csil_v.audit_id)));
    csil_entries.push((cbor_text("operation"), cbor_text(&csil_v.operation)));
    csil_entries.push((
        cbor_text("principal"),
        csil_enc_person_summary(&csil_v.principal),
    ));
    if let Some(csil_inner) = &csil_v.application {
        csil_entries.push((cbor_text("application"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditEntry from a decoded CBOR value tree.
fn csil_dec_audit_entry(csil_root: &CsilCborValue) -> Result<AuditEntry, CsilCborError> {
    let audit_id = {
        let csil_field = cbor_require(csil_root, "audit_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let at = {
        let csil_field = cbor_require(csil_root, "at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let actor = {
        let csil_field = cbor_require(csil_root, "actor")?;
        let csil_decode = csil_dec_person_summary;
        csil_decode(csil_field)?
    };
    let principal = {
        let csil_field = cbor_require(csil_root, "principal")?;
        let csil_decode = csil_dec_person_summary;
        csil_decode(csil_field)?
    };
    let via = {
        let csil_field = cbor_require(csil_root, "via")?;
        let csil_decode = csil_dec_acting_via;
        csil_decode(csil_field)?
    };
    let application = match cbor_map_get(csil_root, "application") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let operation = {
        let csil_field = cbor_require(csil_root, "operation")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let target = match cbor_map_get(csil_root, "target") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let outcome = {
        let csil_field = cbor_require(csil_root, "outcome")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(AuditEntry {
        audit_id,
        at,
        actor,
        principal,
        via,
        application,
        operation,
        target,
        outcome,
    })
}

/// Encode a AuditEntry to canonical CSIL CBOR bytes.
pub fn encode_audit_entry(csil_v: &AuditEntry) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_entry(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditEntry.
pub fn decode_audit_entry(csil_data: &[u8]) -> Result<AuditEntry, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_entry(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditQuery.
fn csil_enc_audit_query(csil_v: &AuditQuery) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.delegated_only {
        csil_entries.push((cbor_text("delegated_only"), cbor_bool(*csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditQuery from a decoded CBOR value tree.
fn csil_dec_audit_query(csil_root: &CsilCborValue) -> Result<AuditQuery, CsilCborError> {
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let delegated_only = match cbor_map_get(csil_root, "delegated_only") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(AuditQuery {
        limit,
        delegated_only,
    })
}

/// Encode a AuditQuery to canonical CSIL CBOR bytes.
pub fn encode_audit_query(csil_v: &AuditQuery) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_query(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditQuery.
pub fn decode_audit_query(csil_data: &[u8]) -> Result<AuditQuery, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_query(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditList.
fn csil_enc_audit_list(csil_v: &AuditList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("entries"),
        cbor_enc_array(&csil_v.entries, csil_enc_audit_entry),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditList from a decoded CBOR value tree.
fn csil_dec_audit_list(csil_root: &CsilCborValue) -> Result<AuditList, CsilCborError> {
    let entries = {
        let csil_field = cbor_require(csil_root, "entries")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_audit_entry);
        csil_decode(csil_field)?
    };
    Ok(AuditList { entries })
}

/// Encode a AuditList to canonical CSIL CBOR bytes.
pub fn encode_audit_list(csil_v: &AuditList) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditList.
pub fn decode_audit_list(csil_data: &[u8]) -> Result<AuditList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_list(&csil_root)
}

/// Build the canonical CBOR value tree for a CompleteRequest.
fn csil_enc_complete_request(csil_v: &CompleteRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.domain {
        csil_entries.push((cbor_text("domain"), cbor_text(csil_inner)));
    }
    csil_entries.push((
        cbor_text("encrypted_token"),
        cbor_text(&csil_v.encrypted_token),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a CompleteRequest from a decoded CBOR value tree.
fn csil_dec_complete_request(csil_root: &CsilCborValue) -> Result<CompleteRequest, CsilCborError> {
    let encrypted_token = {
        let csil_field = cbor_require(csil_root, "encrypted_token")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let domain = match cbor_map_get(csil_root, "domain") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(CompleteRequest {
        encrypted_token,
        domain,
    })
}

/// Encode a CompleteRequest to canonical CSIL CBOR bytes.
pub fn encode_complete_request(csil_v: &CompleteRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_complete_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a CompleteRequest.
pub fn decode_complete_request(csil_data: &[u8]) -> Result<CompleteRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_complete_request(&csil_root)
}

/// Build the canonical CBOR value tree for a LoginResponse.
fn csil_enc_login_response(csil_v: &LoginResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("via"), csil_enc_acting_via(&csil_v.via)));
    csil_entries.push((cbor_text("user"), csil_enc_person_summary(&csil_v.user)));
    csil_entries.push((cbor_text("token"), cbor_text(&csil_v.token)));
    if let Some(csil_inner) = &csil_v.scopes {
        csil_entries.push((
            cbor_text("scopes"),
            cbor_enc_array(csil_inner, csil_enc_scope),
        ));
    }
    if let Some(csil_inner) = &csil_v.acting_as {
        csil_entries.push((cbor_text("acting_as"), csil_enc_person_summary(csil_inner)));
    }
    csil_entries.push((cbor_text("expires_at"), cbor_text(&csil_v.expires_at)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a LoginResponse from a decoded CBOR value tree.
fn csil_dec_login_response(csil_root: &CsilCborValue) -> Result<LoginResponse, CsilCborError> {
    let token = {
        let csil_field = cbor_require(csil_root, "token")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let expires_at = {
        let csil_field = cbor_require(csil_root, "expires_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let user = {
        let csil_field = cbor_require(csil_root, "user")?;
        let csil_decode = csil_dec_person_summary;
        csil_decode(csil_field)?
    };
    let acting_as = match cbor_map_get(csil_root, "acting_as") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let via = {
        let csil_field = cbor_require(csil_root, "via")?;
        let csil_decode = csil_dec_acting_via;
        csil_decode(csil_field)?
    };
    let scopes = match cbor_map_get(csil_root, "scopes") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_scope);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(LoginResponse {
        token,
        expires_at,
        user,
        acting_as,
        via,
        scopes,
    })
}

/// Encode a LoginResponse to canonical CSIL CBOR bytes.
pub fn encode_login_response(csil_v: &LoginResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_login_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a LoginResponse.
pub fn decode_login_response(csil_data: &[u8]) -> Result<LoginResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_login_response(&csil_root)
}

/// Build the canonical CBOR value tree for a MeResponse.
fn csil_enc_me_response(csil_v: &MeResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    csil_entries.push((cbor_text("via"), csil_enc_acting_via(&csil_v.via)));
    csil_entries.push((cbor_text("user"), csil_enc_user(&csil_v.user)));
    if let Some(csil_inner) = &csil_v.scopes {
        csil_entries.push((
            cbor_text("scopes"),
            cbor_enc_array(csil_inner, csil_enc_scope),
        ));
    }
    if let Some(csil_inner) = &csil_v.acting_as {
        csil_entries.push((cbor_text("acting_as"), csil_enc_person_summary(csil_inner)));
    }
    csil_entries.push((
        cbor_text("principals"),
        cbor_enc_array(&csil_v.principals, csil_enc_person_summary),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MeResponse from a decoded CBOR value tree.
fn csil_dec_me_response(csil_root: &CsilCborValue) -> Result<MeResponse, CsilCborError> {
    let user = {
        let csil_field = cbor_require(csil_root, "user")?;
        let csil_decode = csil_dec_user;
        csil_decode(csil_field)?
    };
    let acting_as = match cbor_map_get(csil_root, "acting_as") {
        Some(csil_field) => {
            let csil_decode = csil_dec_person_summary;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let via = {
        let csil_field = cbor_require(csil_root, "via")?;
        let csil_decode = csil_dec_acting_via;
        csil_decode(csil_field)?
    };
    let scopes = match cbor_map_get(csil_root, "scopes") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_scope);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let principals = {
        let csil_field = cbor_require(csil_root, "principals")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_person_summary);
        csil_decode(csil_field)?
    };
    Ok(MeResponse {
        user,
        acting_as,
        via,
        scopes,
        principals,
    })
}

/// Encode a MeResponse to canonical CSIL CBOR bytes.
pub fn encode_me_response(csil_v: &MeResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_me_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MeResponse.
pub fn decode_me_response(csil_data: &[u8]) -> Result<MeResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_me_response(&csil_root)
}

/// Build the canonical CBOR value tree for a ExchangeActAsCredentialRequest.
fn csil_enc_exchange_act_as_credential_request(
    csil_v: &ExchangeActAsCredentialRequest,
) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("credential"), cbor_bytes(&csil_v.credential)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ExchangeActAsCredentialRequest from a decoded CBOR value tree.
fn csil_dec_exchange_act_as_credential_request(
    csil_root: &CsilCborValue,
) -> Result<ExchangeActAsCredentialRequest, CsilCborError> {
    let credential = {
        let csil_field = cbor_require(csil_root, "credential")?;
        let csil_decode = cbor_as_bytes;
        csil_decode(csil_field)?
    };
    Ok(ExchangeActAsCredentialRequest { credential })
}

/// Encode a ExchangeActAsCredentialRequest to canonical CSIL CBOR bytes.
pub fn encode_exchange_act_as_credential_request(
    csil_v: &ExchangeActAsCredentialRequest,
) -> Vec<u8> {
    cbor_encode(&csil_enc_exchange_act_as_credential_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ExchangeActAsCredentialRequest.
pub fn decode_exchange_act_as_credential_request(
    csil_data: &[u8],
) -> Result<ExchangeActAsCredentialRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_exchange_act_as_credential_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ScopeSetRequest.
fn csil_enc_scope_set_request(csil_v: &ScopeSetRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    if let Some(csil_inner) = &csil_v.languages {
        csil_entries.push((
            cbor_text("languages"),
            cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem)),
        ));
    }
    csil_entries.push((
        cbor_text("grantee_domain"),
        cbor_text(&csil_v.grantee_domain),
    ));
    csil_entries.push((
        cbor_text("grantee_user_id"),
        cbor_text(&csil_v.grantee_user_id),
    ));
    csil_entries.push((
        cbor_text("grantee_application_id"),
        cbor_text(&csil_v.grantee_application_id),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ScopeSetRequest from a decoded CBOR value tree.
fn csil_dec_scope_set_request(csil_root: &CsilCborValue) -> Result<ScopeSetRequest, CsilCborError> {
    let grantee_domain = {
        let csil_field = cbor_require(csil_root, "grantee_domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_user_id = {
        let csil_field = cbor_require(csil_root, "grantee_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_application_id = {
        let csil_field = cbor_require(csil_root, "grantee_application_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let languages = match cbor_map_get(csil_root, "languages") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(ScopeSetRequest {
        grantee_domain,
        grantee_user_id,
        grantee_application_id,
        languages,
    })
}

/// Encode a ScopeSetRequest to canonical CSIL CBOR bytes.
pub fn encode_scope_set_request(csil_v: &ScopeSetRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_scope_set_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ScopeSetRequest.
pub fn decode_scope_set_request(csil_data: &[u8]) -> Result<ScopeSetRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_scope_set_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ScopeSetResponse.
fn csil_enc_scope_set_response(csil_v: &ScopeSetResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("signed_scope_set"),
        cbor_bytes(&csil_v.signed_scope_set),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ScopeSetResponse from a decoded CBOR value tree.
fn csil_dec_scope_set_response(
    csil_root: &CsilCborValue,
) -> Result<ScopeSetResponse, CsilCborError> {
    let signed_scope_set = {
        let csil_field = cbor_require(csil_root, "signed_scope_set")?;
        let csil_decode = cbor_as_bytes;
        csil_decode(csil_field)?
    };
    Ok(ScopeSetResponse { signed_scope_set })
}

/// Encode a ScopeSetResponse to canonical CSIL CBOR bytes.
pub fn encode_scope_set_response(csil_v: &ScopeSetResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_scope_set_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ScopeSetResponse.
pub fn decode_scope_set_response(csil_data: &[u8]) -> Result<ScopeSetResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_scope_set_response(&csil_root)
}

/// Build the canonical CBOR value tree for a DevUser.
fn csil_enc_dev_user(csil_v: &DevUser) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    if let Some(csil_inner) = &csil_v.handle {
        csil_entries.push((cbor_text("handle"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevUser from a decoded CBOR value tree.
fn csil_dec_dev_user(csil_root: &CsilCborValue) -> Result<DevUser, CsilCborError> {
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let handle = match cbor_map_get(csil_root, "handle") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(DevUser {
        user_id,
        handle,
        display_name,
        domain,
    })
}

/// Encode a DevUser to canonical CSIL CBOR bytes.
pub fn encode_dev_user(csil_v: &DevUser) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_user(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevUser.
pub fn decode_dev_user(csil_data: &[u8]) -> Result<DevUser, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_user(&csil_root)
}

/// Build the canonical CBOR value tree for a DevUsersResponse.
fn csil_enc_dev_users_response(csil_v: &DevUsersResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("users"),
        cbor_enc_array(&csil_v.users, csil_enc_dev_user),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevUsersResponse from a decoded CBOR value tree.
fn csil_dec_dev_users_response(
    csil_root: &CsilCborValue,
) -> Result<DevUsersResponse, CsilCborError> {
    let users = {
        let csil_field = cbor_require(csil_root, "users")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_dev_user);
        csil_decode(csil_field)?
    };
    Ok(DevUsersResponse { users })
}

/// Encode a DevUsersResponse to canonical CSIL CBOR bytes.
pub fn encode_dev_users_response(csil_v: &DevUsersResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_users_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevUsersResponse.
pub fn decode_dev_users_response(csil_data: &[u8]) -> Result<DevUsersResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_users_response(&csil_root)
}

/// Build the canonical CBOR value tree for a DevLoginRequest.
fn csil_enc_dev_login_request(csil_v: &DevLoginRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    if let Some(csil_inner) = &csil_v.handle {
        csil_entries.push((cbor_text("handle"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.user_id {
        csil_entries.push((cbor_text("user_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevLoginRequest from a decoded CBOR value tree.
fn csil_dec_dev_login_request(csil_root: &CsilCborValue) -> Result<DevLoginRequest, CsilCborError> {
    let user_id = match cbor_map_get(csil_root, "user_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let handle = match cbor_map_get(csil_root, "handle") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(DevLoginRequest {
        user_id,
        handle,
        display_name,
    })
}

/// Encode a DevLoginRequest to canonical CSIL CBOR bytes.
pub fn encode_dev_login_request(csil_v: &DevLoginRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_login_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevLoginRequest.
pub fn decode_dev_login_request(csil_data: &[u8]) -> Result<DevLoginRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_login_request(&csil_root)
}

/// Build the canonical CBOR value tree for a LookupPersonRequest.
fn csil_enc_lookup_person_request(csil_v: &LookupPersonRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((
        cbor_text("identity"),
        csil_enc_identity_ref(&csil_v.identity),
    ));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a LookupPersonRequest from a decoded CBOR value tree.
fn csil_dec_lookup_person_request(
    csil_root: &CsilCborValue,
) -> Result<LookupPersonRequest, CsilCborError> {
    let identity = {
        let csil_field = cbor_require(csil_root, "identity")?;
        let csil_decode = csil_dec_identity_ref;
        csil_decode(csil_field)?
    };
    Ok(LookupPersonRequest { identity })
}

/// Encode a LookupPersonRequest to canonical CSIL CBOR bytes.
pub fn encode_lookup_person_request(csil_v: &LookupPersonRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_lookup_person_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a LookupPersonRequest.
pub fn decode_lookup_person_request(
    csil_data: &[u8],
) -> Result<LookupPersonRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_lookup_person_request(&csil_root)
}

/// Build the canonical CBOR value tree for a UpdateProfileRequest.
fn csil_enc_update_profile_request(csil_v: &UpdateProfileRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.timezone {
        csil_entries.push((cbor_text("timezone"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a UpdateProfileRequest from a decoded CBOR value tree.
fn csil_dec_update_profile_request(
    csil_root: &CsilCborValue,
) -> Result<UpdateProfileRequest, CsilCborError> {
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let timezone = match cbor_map_get(csil_root, "timezone") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(UpdateProfileRequest {
        display_name,
        timezone,
    })
}

/// Encode a UpdateProfileRequest to canonical CSIL CBOR bytes.
pub fn encode_update_profile_request(csil_v: &UpdateProfileRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_update_profile_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a UpdateProfileRequest.
pub fn decode_update_profile_request(
    csil_data: &[u8],
) -> Result<UpdateProfileRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_update_profile_request(&csil_root)
}

/// Build the canonical CBOR value tree for a EmptyRequest.
fn csil_enc_empty_request(_csil_v: &EmptyRequest) -> CsilCborValue {
    CsilCborValue::Map(Vec::new())
}

/// Reconstruct a EmptyRequest from a decoded CBOR value tree.
fn csil_dec_empty_request(_csil_root: &CsilCborValue) -> Result<EmptyRequest, CsilCborError> {
    Ok(EmptyRequest {})
}

/// Encode a EmptyRequest to canonical CSIL CBOR bytes.
pub fn encode_empty_request(csil_v: &EmptyRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_empty_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EmptyRequest.
pub fn decode_empty_request(csil_data: &[u8]) -> Result<EmptyRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_empty_request(&csil_root)
}

/// Build the canonical CBOR value tree for a EmptyResponse.
fn csil_enc_empty_response(_csil_v: &EmptyResponse) -> CsilCborValue {
    CsilCborValue::Map(Vec::new())
}

/// Reconstruct a EmptyResponse from a decoded CBOR value tree.
fn csil_dec_empty_response(_csil_root: &CsilCborValue) -> Result<EmptyResponse, CsilCborError> {
    Ok(EmptyResponse {})
}

/// Encode a EmptyResponse to canonical CSIL CBOR bytes.
pub fn encode_empty_response(csil_v: &EmptyResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_empty_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EmptyResponse.
pub fn decode_empty_response(csil_data: &[u8]) -> Result<EmptyResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_empty_response(&csil_root)
}

/// Build the canonical CBOR value tree for a ServiceError.
fn csil_enc_service_error(csil_v: &ServiceError) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("code"), cbor_uint(csil_v.code)));
    csil_entries.push((cbor_text("message"), cbor_text(&csil_v.message)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ServiceError from a decoded CBOR value tree.
fn csil_dec_service_error(csil_root: &CsilCborValue) -> Result<ServiceError, CsilCborError> {
    let code = {
        let csil_field = cbor_require(csil_root, "code")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    let message = {
        let csil_field = cbor_require(csil_root, "message")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ServiceError { code, message })
}

/// Encode a ServiceError to canonical CSIL CBOR bytes.
pub fn encode_service_error(csil_v: &ServiceError) -> Vec<u8> {
    cbor_encode(&csil_enc_service_error(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ServiceError.
pub fn decode_service_error(csil_data: &[u8]) -> Result<ServiceError, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_service_error(&csil_root)
}

/// Encode a ShareRole enum as its bare literal value.
fn csil_enc_share_role(csil_v: &ShareRole) -> CsilCborValue {
    match csil_v {
        ShareRole::FreeBusy => cbor_text("free_busy"),
        ShareRole::Read => cbor_text("read"),
        ShareRole::Write => cbor_text("write"),
        ShareRole::Manage => cbor_text("manage"),
    }
}

/// Decode a bare literal value into a ShareRole enum.
fn csil_dec_share_role(csil_v: &CsilCborValue) -> Result<ShareRole, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "free_busy" => Ok(ShareRole::FreeBusy),
        "read" => Ok(ShareRole::Read),
        "write" => Ok(ShareRole::Write),
        "manage" => Ok(ShareRole::Manage),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown ShareRole value {csil_other:?}"
        ))),
    }
}

/// Encode a AccessRole enum as its bare literal value.
fn csil_enc_access_role(csil_v: &AccessRole) -> CsilCborValue {
    match csil_v {
        AccessRole::None => cbor_text("none"),
        AccessRole::FreeBusy => cbor_text("free_busy"),
        AccessRole::Read => cbor_text("read"),
        AccessRole::Write => cbor_text("write"),
        AccessRole::Manage => cbor_text("manage"),
        AccessRole::Owner => cbor_text("owner"),
    }
}

/// Decode a bare literal value into a AccessRole enum.
fn csil_dec_access_role(csil_v: &CsilCborValue) -> Result<AccessRole, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "none" => Ok(AccessRole::None),
        "free_busy" => Ok(AccessRole::FreeBusy),
        "read" => Ok(AccessRole::Read),
        "write" => Ok(AccessRole::Write),
        "manage" => Ok(AccessRole::Manage),
        "owner" => Ok(AccessRole::Owner),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown AccessRole value {csil_other:?}"
        ))),
    }
}

/// Encode a AccessPath enum as its bare literal value.
fn csil_enc_access_path(csil_v: &AccessPath) -> CsilCborValue {
    match csil_v {
        AccessPath::Owner => cbor_text("owner"),
        AccessPath::Share => cbor_text("share"),
        AccessPath::Delegation => cbor_text("delegation"),
        AccessPath::Attendee => cbor_text("attendee"),
    }
}

/// Decode a bare literal value into a AccessPath enum.
fn csil_dec_access_path(csil_v: &CsilCborValue) -> Result<AccessPath, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "owner" => Ok(AccessPath::Owner),
        "share" => Ok(AccessPath::Share),
        "delegation" => Ok(AccessPath::Delegation),
        "attendee" => Ok(AccessPath::Attendee),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown AccessPath value {csil_other:?}"
        ))),
    }
}

/// Encode a Scope enum as its bare literal value.
fn csil_enc_scope(csil_v: &Scope) -> CsilCborValue {
    match csil_v {
        Scope::Calendars.read => cbor_text("calendars.read"),
        Scope::Freebusy.read => cbor_text("freebusy.read"),
        Scope::Events.write => cbor_text("events.write"),
        Scope::Events.respond => cbor_text("events.respond"),
        Scope::Calendars.manage => cbor_text("calendars.manage"),
    }
}

/// Decode a bare literal value into a Scope enum.
fn csil_dec_scope(csil_v: &CsilCborValue) -> Result<Scope, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "calendars.read" => Ok(Scope::Calendars.read),
        "freebusy.read" => Ok(Scope::Freebusy.read),
        "events.write" => Ok(Scope::Events.write),
        "events.respond" => Ok(Scope::Events.respond),
        "calendars.manage" => Ok(Scope::Calendars.manage),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown Scope value {csil_other:?}"
        ))),
    }
}

/// Encode a CalendarKind enum as its bare literal value.
fn csil_enc_calendar_kind(csil_v: &CalendarKind) -> CsilCborValue {
    match csil_v {
        CalendarKind::Local => cbor_text("local"),
        CalendarKind::Subscription => cbor_text("subscription"),
    }
}

/// Decode a bare literal value into a CalendarKind enum.
fn csil_dec_calendar_kind(csil_v: &CsilCborValue) -> Result<CalendarKind, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "local" => Ok(CalendarKind::Local),
        "subscription" => Ok(CalendarKind::Subscription),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown CalendarKind value {csil_other:?}"
        ))),
    }
}

/// Encode a EventStatus enum as its bare literal value.
fn csil_enc_event_status(csil_v: &EventStatus) -> CsilCborValue {
    match csil_v {
        EventStatus::Confirmed => cbor_text("confirmed"),
        EventStatus::Tentative => cbor_text("tentative"),
        EventStatus::Cancelled => cbor_text("cancelled"),
    }
}

/// Decode a bare literal value into a EventStatus enum.
fn csil_dec_event_status(csil_v: &CsilCborValue) -> Result<EventStatus, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "confirmed" => Ok(EventStatus::Confirmed),
        "tentative" => Ok(EventStatus::Tentative),
        "cancelled" => Ok(EventStatus::Cancelled),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown EventStatus value {csil_other:?}"
        ))),
    }
}

/// Encode a EventTransparency enum as its bare literal value.
fn csil_enc_event_transparency(csil_v: &EventTransparency) -> CsilCborValue {
    match csil_v {
        EventTransparency::Opaque => cbor_text("opaque"),
        EventTransparency::Transparent => cbor_text("transparent"),
    }
}

/// Decode a bare literal value into a EventTransparency enum.
fn csil_dec_event_transparency(csil_v: &CsilCborValue) -> Result<EventTransparency, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "opaque" => Ok(EventTransparency::Opaque),
        "transparent" => Ok(EventTransparency::Transparent),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown EventTransparency value {csil_other:?}"
        ))),
    }
}

/// Encode a EventVisibility enum as its bare literal value.
fn csil_enc_event_visibility(csil_v: &EventVisibility) -> CsilCborValue {
    match csil_v {
        EventVisibility::Default => cbor_text("default"),
        EventVisibility::Public => cbor_text("public"),
        EventVisibility::Private => cbor_text("private"),
        EventVisibility::Confidential => cbor_text("confidential"),
    }
}

/// Decode a bare literal value into a EventVisibility enum.
fn csil_dec_event_visibility(csil_v: &CsilCborValue) -> Result<EventVisibility, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "default" => Ok(EventVisibility::Default),
        "public" => Ok(EventVisibility::Public),
        "private" => Ok(EventVisibility::Private),
        "confidential" => Ok(EventVisibility::Confidential),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown EventVisibility value {csil_other:?}"
        ))),
    }
}

/// Encode a ParticipationStatus enum as its bare literal value.
fn csil_enc_participation_status(csil_v: &ParticipationStatus) -> CsilCborValue {
    match csil_v {
        ParticipationStatus::NeedsAction => cbor_text("needs_action"),
        ParticipationStatus::Accepted => cbor_text("accepted"),
        ParticipationStatus::Declined => cbor_text("declined"),
        ParticipationStatus::Tentative => cbor_text("tentative"),
        ParticipationStatus::Delegated => cbor_text("delegated"),
    }
}

/// Decode a bare literal value into a ParticipationStatus enum.
fn csil_dec_participation_status(
    csil_v: &CsilCborValue,
) -> Result<ParticipationStatus, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "needs_action" => Ok(ParticipationStatus::NeedsAction),
        "accepted" => Ok(ParticipationStatus::Accepted),
        "declined" => Ok(ParticipationStatus::Declined),
        "tentative" => Ok(ParticipationStatus::Tentative),
        "delegated" => Ok(ParticipationStatus::Delegated),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown ParticipationStatus value {csil_other:?}"
        ))),
    }
}

/// Encode a AttendeeRole enum as its bare literal value.
fn csil_enc_attendee_role(csil_v: &AttendeeRole) -> CsilCborValue {
    match csil_v {
        AttendeeRole::Chair => cbor_text("chair"),
        AttendeeRole::Required => cbor_text("required"),
        AttendeeRole::Optional => cbor_text("optional"),
        AttendeeRole::NonParticipant => cbor_text("non_participant"),
    }
}

/// Decode a bare literal value into a AttendeeRole enum.
fn csil_dec_attendee_role(csil_v: &CsilCborValue) -> Result<AttendeeRole, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "chair" => Ok(AttendeeRole::Chair),
        "required" => Ok(AttendeeRole::Required),
        "optional" => Ok(AttendeeRole::Optional),
        "non_participant" => Ok(AttendeeRole::NonParticipant),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown AttendeeRole value {csil_other:?}"
        ))),
    }
}

/// Encode a ReminderMethod enum as its bare literal value.
fn csil_enc_reminder_method(csil_v: &ReminderMethod) -> CsilCborValue {
    match csil_v {
        ReminderMethod::Display => cbor_text("display"),
        ReminderMethod::Email => cbor_text("email"),
    }
}

/// Decode a bare literal value into a ReminderMethod enum.
fn csil_dec_reminder_method(csil_v: &CsilCborValue) -> Result<ReminderMethod, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "display" => Ok(ReminderMethod::Display),
        "email" => Ok(ReminderMethod::Email),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown ReminderMethod value {csil_other:?}"
        ))),
    }
}

/// Encode a RecurrenceScope enum as its bare literal value.
fn csil_enc_recurrence_scope(csil_v: &RecurrenceScope) -> CsilCborValue {
    match csil_v {
        RecurrenceScope::All => cbor_text("all"),
        RecurrenceScope::This => cbor_text("this"),
        RecurrenceScope::ThisAndFuture => cbor_text("this_and_future"),
    }
}

/// Decode a bare literal value into a RecurrenceScope enum.
fn csil_dec_recurrence_scope(csil_v: &CsilCborValue) -> Result<RecurrenceScope, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "all" => Ok(RecurrenceScope::All),
        "this" => Ok(RecurrenceScope::This),
        "this_and_future" => Ok(RecurrenceScope::ThisAndFuture),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown RecurrenceScope value {csil_other:?}"
        ))),
    }
}

/// Encode a ActingVia enum as its bare literal value.
fn csil_enc_acting_via(csil_v: &ActingVia) -> CsilCborValue {
    match csil_v {
        ActingVia::Self => cbor_text("self"),
        ActingVia::Delegation => cbor_text("delegation"),
        ActingVia::ActAsGrant => cbor_text("act_as_grant"),
    }
}

/// Decode a bare literal value into a ActingVia enum.
fn csil_dec_acting_via(csil_v: &CsilCborValue) -> Result<ActingVia, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "self" => Ok(ActingVia::Self),
        "delegation" => Ok(ActingVia::Delegation),
        "act_as_grant" => Ok(ActingVia::ActAsGrant),
        csil_other => Err(CsilCborError(format!(
            "csil cbor: unknown ActingVia value {csil_other:?}"
        ))),
    }
}

/// Encode the calendar_get_calendar_request payload to canonical CSIL CBOR bytes.
pub fn encode_calendar_get_calendar_request(csil_v: &CalendarID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the calendar_get_calendar_request payload.
pub fn decode_calendar_get_calendar_request(csil_data: &[u8]) -> Result<CalendarID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the calendar_delete_calendar_request payload to canonical CSIL CBOR bytes.
pub fn encode_calendar_delete_calendar_request(csil_v: &CalendarID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the calendar_delete_calendar_request payload.
pub fn decode_calendar_delete_calendar_request(
    csil_data: &[u8],
) -> Result<CalendarID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the calendar_refresh_subscription_request payload to canonical CSIL CBOR bytes.
pub fn encode_calendar_refresh_subscription_request(csil_v: &CalendarID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the calendar_refresh_subscription_request payload.
pub fn decode_calendar_refresh_subscription_request(
    csil_data: &[u8],
) -> Result<CalendarID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the share_list_shares_request payload to canonical CSIL CBOR bytes.
pub fn encode_share_list_shares_request(csil_v: &CalendarID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the share_list_shares_request payload.
pub fn decode_share_list_shares_request(csil_data: &[u8]) -> Result<CalendarID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the share_delete_share_request payload to canonical CSIL CBOR bytes.
pub fn encode_share_delete_share_request(csil_v: &ShareID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the share_delete_share_request payload.
pub fn decode_share_delete_share_request(csil_data: &[u8]) -> Result<ShareID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the delegation_revoke_delegation_request payload to canonical CSIL CBOR bytes.
pub fn encode_delegation_revoke_delegation_request(csil_v: &DelegationID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the delegation_revoke_delegation_request payload.
pub fn decode_delegation_revoke_delegation_request(
    csil_data: &[u8],
) -> Result<DelegationID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the event_get_event_request payload to canonical CSIL CBOR bytes.
pub fn encode_event_get_event_request(csil_v: &EventID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the event_get_event_request payload.
pub fn decode_event_get_event_request(csil_data: &[u8]) -> Result<EventID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}
