package temper

import (
	"context"
	"time"
)

// ProcessesService manages background processes in an environment. Processes are tracked in VM memory: they
// survive suspend and resume, but can no longer be found after the environment moves to a different VM.
type ProcessesService struct{ c *Client }

// Spawn starts a background process (201) without waiting for it to finish. Each environment can have at most 64
// (including exited ones that haven't been deleted); beyond that it returns too_many_processes.
func (s *ProcessesService) Spawn(ctx context.Context, id string, p SpawnParams) (*Process, error) {
	var out Process
	return &out, s.c.do(ctx, request{method: "POST", path: envPath(id, "/processes"), json: p, timeout: shortCall}, &out)
}

// List lists all background processes (including exited ones that haven't been deleted).
func (s *ProcessesService) List(ctx context.Context, id string) ([]Process, error) {
	var out struct {
		Data []Process `json:"data"`
	}
	return out.Data, s.c.do(ctx, request{method: "GET", path: envPath(id, "/processes"), timeout: shortCall}, &out)
}

// Get returns a background process.
func (s *ProcessesService) Get(ctx context.Context, id, pid string) (*Process, error) {
	var out Process
	return &out, s.c.do(ctx, request{method: "GET", path: envPath(id, "/processes/"+seg(pid)), timeout: shortCall}, &out)
}

// Output fetches output with long polling, starting at offset since: output that is already available is returned
// immediately; otherwise it waits up to waitSecs seconds (0–30).
func (s *ProcessesService) Output(ctx context.Context, id, pid string, since int64, waitSecs int) (*ProcessOutput, error) {
	var out ProcessOutput
	query := newQuery().int64("since", since).int("wait_secs", waitSecs).values()
	return &out, s.c.do(ctx, request{
		method: "GET", path: envPath(id, "/processes/"+seg(pid)+"/output"), query: query,
		timeout: shortCall + time.Duration(waitSecs)*time.Second,
	}, &out)
}

// Follow long-polls until the process exits, calling onChunk for each chunk of output (it stops if onChunk returns
// an error), and then returns the exit status. It starts at since (0 = from the beginning); if it is interrupted,
// you can continue by passing the last chunk's Offset + len(Data) as since.
func (s *ProcessesService) Follow(ctx context.Context, id, pid string, since int64, onChunk func(OutputChunk) error) (*ProcessExit, error) {
	for {
		page, err := s.Output(ctx, id, pid, since, 30)
		if err != nil {
			return nil, err
		}
		for _, c := range page.Chunks {
			if err := onChunk(c); err != nil {
				return nil, err
			}
		}
		since = page.Next
		if page.Exited {
			return page.Exit, nil
		}
		if !page.Process.Running && len(page.Chunks) == 0 {
			return page.Process.Exit, nil
		}
	}
}

// Signal sends a signal: TERM, KILL, INT or HUP.
func (s *ProcessesService) Signal(ctx context.Context, id, pid, signal string) error {
	return s.c.do(ctx, request{
		method: "POST", path: envPath(id, "/processes/"+seg(pid)+"/signal"), json: map[string]string{"signal": signal}, timeout: shortCall,
	}, nil)
}

// Delete kills the process if it is still running, then deletes its record and retained output.
func (s *ProcessesService) Delete(ctx context.Context, id, pid string) error {
	return s.c.do(ctx, request{method: "DELETE", path: envPath(id, "/processes/"+seg(pid)), timeout: shortCall}, nil)
}
