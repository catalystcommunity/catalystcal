package csilrpc

import "context"

// TypedHandler is the dispatch shape a generated-interface method is adapted to:
// it decodes a request payload with the generated codec, invokes the typed
// service method, and returns the success-arm variant name plus the encoded
// response payload — or an error. An *Error becomes the declared ServiceError
// arm (status 0); any other error is a transport-internal failure.
//
// The payload codec is the generated per-op codec (csil.Encode<Service><Op>… /
// csil.Decode<Service><Op>…), so the generated code owns the wire bytes.
type TypedHandler func(ctx context.Context, payload []byte) (variant string, out []byte, err error)

// Route adapts a typed generated-interface method into a TypedHandler using the
// generated per-op decode/encode functions. The success arm is named from the
// response value (successVariant), so reflection touches only the small variant
// string, never the payload bytes.
//
// Decode failures surface as a caller-visible BadRequest (the body shape is the
// caller's concern); the method's own *Error / error pass through unchanged.
func Route[Req any, Resp any](
	fn func(context.Context, Req) (Resp, error),
	decode func([]byte) (Req, error),
	encode func(Resp) []byte,
) TypedHandler {
	return func(ctx context.Context, payload []byte) (string, []byte, error) {
		req, err := decode(payload)
		if err != nil {
			return "", nil, BadRequest("invalid CBOR body: " + err.Error())
		}
		resp, err := fn(ctx, req)
		if err != nil {
			return "", nil, err
		}
		return successVariant(resp), encode(resp), nil
	}
}
