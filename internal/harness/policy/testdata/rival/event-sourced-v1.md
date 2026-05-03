# Contract: Authorize purchase command for marketplace order

## Intent
- Reject purchase commands for which the customer's wallet balance is insufficient.
- Success means rejected purchases emit a PurchaseRejected event and accepted purchases emit a PurchaseAuthorized event.

## Domain
- Command: AuthorizePurchase will charge the wallet aggregate and append events to its stream.
- Event: PurchaseAuthorized did record a successful debit; PurchaseRejected did record an insufficient-funds outcome.
- Read model: WalletBalanceProjection projects the running balance for query handlers.
- Aggregate: Wallet (aggregate root) replays events to compute current balance before deciding authorization.
- Policy: WHEN PurchaseRejected THEN NotifyCustomerOfDecline COMMAND.

## Decisions
- Authorize purchases inside the wallet aggregate, never inside HTTP handlers.
- Persist events first, then update the projection; rebuild the projection from the event stream on demand.

## Steps
1. Add AuthorizePurchase command handler to wallet aggregate.
   - Target: `internal/wallet/aggregate.go`
   - Acceptance: rejected purchases append PurchaseRejected; accepted purchases append PurchaseAuthorized.
2. Add WalletBalanceProjection event handler.
   - Target: `internal/wallet/projection.go`
   - Acceptance: projection state matches sum of authorized minus rejected events.

## Boundaries
- Do not mutate wallet balance directly; balance is derived from events only.
- Do not couple the projection to the aggregate's mutable state.
- Do not introduce ambient context.Context inside the domain layer.

## Evidence
- test: just test
- lint: just lint
- nfr.p95_latency_ms: <= 200
- semgrep: no aggregate→external imports
