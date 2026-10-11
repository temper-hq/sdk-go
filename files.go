package temper

import (
	"context"
	"io"
	"time"
)

// FilesService accesses files in an environment. Paths are paths inside the runtime unit; relative paths are
// resolved against the agent user's home directory, and permissions are the same as the agent's. An environment
// that isn't running is woken first. Reading and writing files have no default timeout (files can be large); use
// the context to control them. Listing and deleting default to 120 seconds (they may have to wait for a wake-up).
type FilesService struct{ c *Client }

// Open reads a file and returns a stream (the caller must Close it). It returns not_found if the file doesn't
// exist, bad_request if the path is a directory, and forbidden if the file can't be read.
func (s *FilesService) Open(ctx context.Context, id, path string) (io.ReadCloser, error) {
	res, cancel, err := s.c.send(ctx, request{
		method: "GET", path: envPath(id, "/files"), query: newQuery().str("path", path).values(),
		accept: "application/octet-stream, application/json", timeout: -1,
	})
	if err != nil {
		return nil, err
	}
	return cancelOnClose{res.Body, cancel}, nil
}

// Read reads the whole file.
func (s *FilesService) Read(ctx context.Context, id, path string) ([]byte, error) {
	r, err := s.Open(ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Write writes a file (overwriting it; up to 1 GiB), streaming body as the upload. It returns not_found if the
// parent directory doesn't exist; set mkdir to true to create the parent directories first.
func (s *FilesService) Write(ctx context.Context, id, path string, body io.Reader, mkdir bool) (*WriteFileResult, error) {
	var out WriteFileResult
	return &out, s.c.do(ctx, request{
		method: "PUT", path: envPath(id, "/files"), query: newQuery().str("path", path).boolean("mkdir", mkdir).values(),
		body: body, ctype: "application/octet-stream", timeout: -1,
	}, &out)
}

// List lists a directory (one level, sorted by name, at most 10,000 entries).
func (s *FilesService) List(ctx context.Context, id, path string) (*DirectoryListing, error) {
	var out DirectoryListing
	return &out, s.c.do(ctx, request{
		method: "GET", path: envPath(id, "/files/list"), query: newQuery().str("path", path).values(), timeout: shortCall,
	}, &out)
}

// Delete deletes a file; deleting a directory requires recursive. It returns not_found if the path doesn't exist.
func (s *FilesService) Delete(ctx context.Context, id, path string, recursive bool) error {
	return s.c.do(ctx, request{
		method: "DELETE", path: envPath(id, "/files"), query: newQuery().str("path", path).boolean("recursive", recursive).values(),
		timeout: shortCall,
	}, nil)
}

// shortCall is the timeout for short exec and file calls, which may also have to wait for a wake-up.
const shortCall = wakeBudget + 15*time.Second
