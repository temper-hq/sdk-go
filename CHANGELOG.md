# Changelog

Go SDK (module `github.com/temper-hq/sdk-go`, package `temper`). Versions follow the API spec's `info.version` (`temper.Version`).
Before 1.0, minor versions may include breaking changes; they are noted here.

## 0.6.0 — 2026-10-18

Added:

- Ingress: every environment has a public HTTPS address (`IngressRouteList.Host`). `Environments.ListIngressRoutes`,
  `SetIngressRoute` and `DeleteIngressRoute` manage the routes on it (`IngressRouteSpec`: path, verification by Standard
  Webhooks, Slack, Telegram or Stripe signature, an immediate response or `IngressRespond{Forward: true}`, and the port
  inside the VM to deliver to); a request wakes the environment and is retried for 24 hours.
  `Environments.ListIngressDeliveries` lists recent requests and their delivery state. When a Standard Webhooks route is
  registered without a secret the platform generates one and returns it once in `IngressRoute.Secret`.
- Egress modes: `Secrets.GetEgressMode` / `SetEgressMode` (developer level) and `SetEnvironmentEgressMode` decide what
  happens to hosts outside the allowlist: `EgressAllowlist` (refuse, the default), `EgressOpen` (allow and audit),
  `EgressAsk` (hold the connection while an approval with connector `egress:<host>` and category `connect` is requested;
  a permanent approval adds the host to the environment's allowlist) or `EgressHook` (ask the endpoint set with
  `Secrets.SetEgressHook` first). `EnvironmentEgress` has `Mode` and `ModeInherited`.
- Templates: `CreateTemplateVersionParams.FromImage` starts a version from an OCI image instead of a base template, and
  `RegistrySecret` names the developer-level secret used to pull a private one; `TemplateVersion` has `FromImage`,
  `RegistrySecret` and `ImageDigest`.
- `Environments.AgentLogs` returns a slice of the Agent's output together with its state (`AgentStatus`: state, restarts,
  last exit); pass `Next` back as `Since` to follow it.
- Snapshots of an environment's data disk: `Environments.CreateSnapshot`, `ListSnapshots`, `GetSnapshot`,
  `DeleteSnapshot`, and `Environments.Restore` to roll the disk back to one. `CreateEnvironmentParams.FromSnapshot` branches
  a new environment from a snapshot. `Environment` has `SourceSnapshotID`, `SourceEnvironmentID` and `Restoring`;
  `UsageTotals` has `Snapshots`, `Forks` and `SnapshotGiBHours`.

Breaking:

- `Me.Developers` and the `Team` type are gone: a console session now identifies a person, not a team, so there is no team list
  to switch between. `Me.Organization` (`ID`, `Name`, `Slug`) is the organization the API key's project belongs to.
  `Me.Developer` is still the key's project.

## 0.5.0 — 2026-10-11

First release of the Go SDK. It covers the same API as the TypeScript and Python SDKs, except WebSocket attach for
background processes (use `Processes.Follow` with long polling instead).

- Client: `New` with the options `WithAPIKey`, `WithBaseURL`, `WithHTTPClient`, `WithHeader` and `WithTimeout`; reads
  `TEMPER_API_KEY` and `TEMPER_BASE_URL` by default. Every method takes a `context.Context`. Depends only on the
  standard library.
- `Environments`: create (with `IdempotencyKey`), batch create (`CreateBatch`), get, list (`List`, `Iter`), `Suspend`,
  `Resume`, `Wake`, `KeepAwake`, `Destroy`, events (`Events`, `IterEvents`), idle ladder (`UpdateIdle`), scheduled
  wakeups (`CreateWakeup`, `ListWakeups`, `CancelWakeup`), schedules (`ListSchedules`, `CancelSchedule`), agent rules and
  quotas (`UpdateAgentSettings`), non-secret config (`GetConfig`, `SetConfig`), `WaitFor` and `Exec`.
- `Files`: `Open` (stream), `Read`, `Write` (streamed from an `io.Reader`), `List`, `Delete`.
- `Processes`: `Spawn`, `List`, `Get`, `Output` (long polling), `Follow`, `Signal`, `Delete`.
- `Agent`: package upload (`UploadPackage`, streamed), `ListPackages`, `SetDefaultVersion`, and staged rollouts
  (`StartRollout`, `ListRollouts`, `GetRollout`, `WidenRollout`, `PauseRollout`, `ResumeRollout`, `RollbackRollout`).
- `Browser`: profiles (`CreateProfile`, `ListProfiles`, `GetProfile`, `DeleteProfile`), leases (`AcquireLease`,
  `ListLeases`, `IterLeases`, `GetLease`, `RenewLease`, `ReleaseLease`), end-user takeover (`Takeover`, `Handback`) and
  `Limits`.
- `Connections`: per-user OAuth (`Authorize`, `List`, `Get`, `Revoke`) and your own OAuth apps (`ListOAuthClients`,
  `SetOAuthClient`).
- `Policies`: developer default policy and per-environment policies (`GetDefault`, `SetDefault`, `DeleteDefault`,
  `GetForEnvironment`, `SetForEnvironment`, `DeleteForEnvironment`), and a read-only `Check`.
- `Approvals`: approval requests (`List`, `Iter`, `Get`, `Approve`, `Deny`) and grants (`ListGrants`, `RevokeGrant`);
  scope helpers `OnceScope`, `SessionScope`, `TaskScope`, `UntilScope`, `PermanentScope`.
- `Audit`: audit records (`List`, `Iter`), raw row export (`Export`, NDJSON stream) and hash chain status (`Chains`).
- `Metering`: usage (`Usage`) and monthly limits (`ListLimits`, `SetLimit`, `DeleteLimit`).
- `Team`: current identity (`Me`), members, API keys and webhook settings.
- `Secrets`: developer- and environment-level credentials (values are never returned) and the egress allowlist,
  including `AddEnvironmentEgressHost` / `RemoveEnvironmentEgressHost` for single hosts.
- `Templates`: environment templates (`CreateVersion`, `GetVersion`, `List`, `DeleteVersion`, `WaitForVersion`), step
  helpers (`RunStep`, `EnvStep`, `WorkdirStep`, `CopyStep`, `CopyBytes`, `StartShell`, `StartArgv`) and `Template` on
  `CreateEnvironmentParams`.
- Webhooks: `VerifyWebhook`, `SignWebhook`, `SignatureHeader`, and the `ApprovalRequested` payload type.
- Types: `Environment.OverLimit`, `BrowserLease.TakeoverStartedAt`.
- Errors: `*temper.Error` and `IsCode`, with constants for common codes, including `CodeNoOAuthClient`,
  `CodeAlreadyDecided`, `CodeMissingScope`, `CodeInvalidScope`, `CodeScopeTooWide`, `CodeLimitExceeded` and
  `CodeExportTooLarge`.
