# Temper Go SDK

Go client for the Temper control plane REST API. Requires Go 1.21+ and depends only on the standard library.

Documentation: [SDK guide](https://docs.temper.im/getting-started/sdks/) · [API reference](https://docs.temper.im/api/)

```bash
go get github.com/temper-hq/sdk-go
```

```go
import temper "github.com/temper-hq/sdk-go"
```

## Quick start

```go
ctx := context.Background()
client, err := temper.New() // reads TEMPER_API_KEY and TEMPER_BASE_URL; or pass temper.WithAPIKey(...), temper.WithBaseURL(...)
if err != nil { log.Fatal(err) }

env, err := client.Environments.Create(ctx, temper.CreateEnvironmentParams{EndUserID: "user_42"})
if temper.IsCode(err, temper.CodeEnvironmentExists) {
	// already exists
}

r, err := client.Environments.Exec(ctx, env.ID, temper.ExecParams{Command: "python3 --version", TimeoutSecs: 30})
fmt.Println(*r.ExitCode, r.Stdout)

// Background process: follow it to the end with long polling
p, _ := client.Processes.Spawn(ctx, env.ID, temper.SpawnParams{Command: "npm test"})
exit, err := client.Processes.Follow(ctx, env.ID, p.ID, 0, func(c temper.OutputChunk) error {
	fmt.Print(c.Data)
	return nil
})

// Pagination
it := client.Environments.Iter(temper.ListEnvironmentsParams{State: "running"})
for it.Next(ctx) {
	fmt.Println(it.Value().ID)
}
if err := it.Err(); err != nil { log.Fatal(err) }
```

For step-by-step examples in every SDK language (creating an environment, exec and files, connecting Gmail, approval
webhooks, audit and usage), see the [docs](https://docs.temper.im/).

## API

| Field | Methods |
| --- | --- |
| `Environments` | `Create`, `CreateBatch`, `Get`, `GetConfig`, `SetConfig`, `WaitFor`, `List`, `Iter`, `Suspend`, `Resume`, `Wake`, `KeepAwake`, `Destroy`, `UpdateIdle`, `Events`, `IterEvents`, `Exec`, `CreateWakeup`, `ListWakeups`, `CancelWakeup`, `ListSchedules`, `CancelSchedule`, `UpdateAgentSettings` |
| `Files` | `Open` (stream), `Read`, `Write` (`io.Reader`, streamed upload), `List`, `Delete` |
| `Processes` | `Spawn`, `List`, `Get`, `Output` (long polling), `Follow`, `Signal`, `Delete` |
| `Connections` | `Authorize`, `List`, `Get`, `Revoke`, `ListOAuthClients`, `SetOAuthClient` |
| `Policies` | `GetDefault`, `SetDefault`, `DeleteDefault`, `GetForEnvironment`, `SetForEnvironment`, `DeleteForEnvironment`, `Check` |
| `Approvals` | `List`, `Iter`, `Get`, `Approve`, `Deny`, `ListGrants`, `RevokeGrant`; scopes: `OnceScope`, `SessionScope`, `TaskScope`, `UntilScope`, `PermanentScope` |
| `Audit` | `List`, `Iter`, `Export` (NDJSON stream), `Chains` |
| `Metering` | `Usage`, `ListLimits`, `SetLimit`, `DeleteLimit` |
| `Agent` | `UploadPackage` (`io.Reader`, streamed; also reports whether the package was newly created), `ListPackages`, `SetDefaultVersion`, `StartRollout`, `ListRollouts`, `GetRollout`, `WidenRollout`, `PauseRollout`, `ResumeRollout`, `RollbackRollout` |
| `Browser` | `CreateProfile`, `ListProfiles`, `GetProfile`, `DeleteProfile`, `AcquireLease`, `ListLeases`, `IterLeases`, `GetLease`, `RenewLease`, `ReleaseLease`, `Takeover`, `Handback`, `Limits` |
| `Secrets` | `List`, `Set`, `Delete`, `ListForEnvironment`, `SetForEnvironment`, `DeleteForEnvironment` (credentials: values are never returned, and the VM only sees a placeholder); `GetEgress`, `SetEgress`, `GetEnvironmentEgress`, `SetEnvironmentEgress`, `AddEnvironmentEgressHost`, `RemoveEnvironmentEgressHost` (egress allowlist) |
| `Team` | `Me`, `ListMembers`, `AddMember`, `SetMemberRole`, `RemoveMember`, `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey`, `GetWebhook`, `SetWebhook` (adding, changing or removing members, issuing keys and changing the webhook require a console session; calling them with an API key returns `session_required`) |
| `Templates` | `CreateVersion`, `GetVersion`, `List`, `DeleteVersion`, `WaitForVersion`; step helpers `RunStep`, `EnvStep`, `WorkdirStep`, `CopyStep`, `CopyBytes`, `StartShell`, `StartArgv` |
| Functions | `VerifyWebhook`, `SignWebhook`, `IsCode`, `Ptr` |

The Go SDK covers the same API as the TypeScript and Python SDKs, except WebSocket attach for background processes
(it would need a third-party library); use `Processes.Follow` with long polling instead.

- **context**: every method takes a `context.Context` as its first argument. Without a deadline, the default timeout
  applies: 30 seconds for most calls (change it with `WithTimeout`); `Exec` uses 105 seconds for waking + `TimeoutSecs`
  + 30 seconds; file reads and writes and audit export have no timeout; other exec and file calls use 120 seconds.
- **Optional fields**: fields where 0 is meaningful are pointers (`IdleSettings{SuspendAfterSecs: temper.Ptr(0)}` means
  "never suspend automatically"); other zero values are not sent.
- **Times**: responses use `time.Time` (`*time.Time` where the value may be empty); times in query parameters are sent
  as RFC 3339 in UTC.
- **Time-limited grants**: `UntilScope(t)` is written as `{"kind": "until", "until": <unix seconds>}` in the decision
  request, and called `time_limited` in stored grants and in `MaxScope`.
- **Environments**: set `CreateEnvironmentParams.IdempotencyKey` to retry a create safely after a timeout.
  `WaitFor(ctx, id, "running", nil)` polls until the environment reaches the state (typically after `Resume`).

## Errors

Non-2xx responses return a `*temper.Error` with `Status` (the HTTP status; 0 for network errors and timeouts), `Code`
(the `error` field of the control plane error body, such as `not_found`, `limit_exceeded` or `invalid_scope`;
`http_<status>` for non-JSON error responses) and `Message`. Check it with `temper.IsCode(err, temper.CodeXxx)` or
`errors.As(err, &te)`. Network errors have code `network_error`, timeouts `timeout` and cancellations `aborted`;
`Unwrap()` returns the underlying error.

## Webhooks

```go
body, _ := io.ReadAll(r.Body) // always use the raw request body
ev, err := temper.VerifyWebhook(secret, body, r.Header.Get(temper.SignatureHeader), 0, time.Time{})
if err != nil { w.WriteHeader(400); return }
if ev.Type == "approval.requested" {
	var a temper.ApprovalRequested
	_ = json.Unmarshal(ev.Data, &a)
	client.Approvals.Approve(ctx, a.ApprovalID, temper.OnceScope())
}
```

The default tolerance for clock skew is 5 minutes. The header may carry several `v1` signatures (after the signing
secret is rotated, both the old and new ones are present for 24 hours).

## Versioning

The version (`temper.Version`) follows the API spec's `info.version`. See [CHANGELOG.md](CHANGELOG.md).

## License

Apache-2.0. See LICENSE.
