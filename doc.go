// Package temper is the Go client for the Temper control plane REST API. The full API is described in the Temper
// API reference (https://docs.temper.im/api/).
//
//	client, err := temper.New() // reads TEMPER_API_KEY and TEMPER_BASE_URL
//	env, err := client.Environments.Create(ctx, temper.CreateEnvironmentParams{EndUserID: "alice"})
//	r, err := client.Environments.Exec(ctx, env.ID, temper.ExecParams{Command: "python3 --version"})
//
// Every method takes a context.Context as its first argument; use it for cancellation and timeouts. When the
// context has no deadline, the client's default timeout applies (30 seconds for most calls; exec and file calls
// have to wait for the environment to wake up and default to longer timeouts, see each method).
//
// Errors are returned as *Error, which carries the HTTP status and the control plane's error code:
//
//	if temper.IsCode(err, temper.CodeEnvironmentExists) { ... }
//
// The package depends only on the standard library.
package temper
