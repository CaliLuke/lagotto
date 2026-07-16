# G1E — Foreign Holder

A broad holder type has been structurally decomposed, but production code in
other packages still accepts the holder and reaches through it to narrower
services. The packages moved; the caller contract did not.

## Why this matters

Producer-side checks can confirm that sub-services are real types in real
subpackages. They cannot prove that consumers stopped depending on the old
chokepoint. If every handler still accepts `*Store`, changes to construction,
lifetime, and dependency wiring continue to ripple through the whole system.

## What lagotto checks

Lagotto first identifies a holder with either:

- at least three pointer fields whose types live in its subpackages, or
- at least three exported accessors that take the holder and return a service
  from one of its subpackages.

It then scans other production packages for that holder in function or method
parameters, non-constructor return types, and exported struct fields. Test
fixture packages and conventional `New`/`Open`/`Make`/`Build`/`Create`
constructors are exempt.

| Severity | Condition |
| --- | --- |
| CRITICAL | At least five signature sites across at least three packages |
| HIGH | Any production signature in a foreign package |

## Example

```go
// package store
type Store struct {
    Users   *users.Service
    Billing *billing.Service
    Reports *reports.Service
}

// package transport — G1E
func HandleCreate(store *store.Store, request Request) error {
    return store.Users.Create(request.User)
}
```

Change `HandleCreate` to accept `*users.Service`. Construction code may still
assemble the services, but production consumers should receive only the narrow
capability they use.

## Related

G1E is the consumer-side complement to [G1C Aggregate
Holder](g1c-aggregate-holder.md) and [G1D Hidden
Holder](g1d-hidden-holder.md).
