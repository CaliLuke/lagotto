// Package audit defines the wire types every detector emits and the
// shared serializer that renders them as JSON or text.
//
// Finding is the per-smell record. Report is the top-level envelope.
// Severity ranks findings from CRITICAL down to LOW; Emit sorts and
// writes a Report to stdout.
package audit
