package sync

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/igungor/go-putio/putio"
)

// Tasks stores active tasks.
type Tasks struct {
	sync.Mutex
	s map[int64]*task
}

func NewTasks() *Tasks {
	return &Tasks{s: make(map[int64]*task)}
}

func (m *Tasks) Add(t *task) {
	m.Lock()
	defer m.Unlock()

	m.s[t.f.ID] = t
}

func (m *Tasks) Remove(t *task) {
	m.Lock()
	defer m.Unlock()

	delete(m.s, t.f.ID)
}

func (m *Tasks) Exists(t *task) bool {
	m.Lock()
	defer m.Unlock()

	_, ok := m.s[t.f.ID]
	return ok
}

func (m *Tasks) Empty() bool {
	m.Lock()
	defer m.Unlock()
	return len(m.s) == 0
}

// chunk represents file chunks. Files can be split into pieces and downloaded
// with multiple connections, each connection fetches a part of a file.
type chunk struct {
	// Where the chunk starts
	offset int64

	// Length of chunk
	length int64
}

func (c chunk) String() string {
	return fmt.Sprintf("chunk{%v-%v}", c.offset, c.offset+c.length)
}

type task struct {
	f      putio.File
	cwd    string
	state  *State
	chunks []*chunk
}

func (t task) String() string {
	return fmt.Sprintf("task<name: %q, size: %v, chunks: %v, bitfield: %v>",
		trimPath(path.Join(t.cwd, t.f.Name)),
		t.f.Size,
		t.chunks,
		t.state.Bitfield.Len(),
	)
}

// verify checks bitfield integrity and computes CRC32 of the given task.
func verify(r io.Reader, task *task) error {
	if !task.state.Bitfield.All() {
		return fmt.Errorf("Not all bits are downloaded for file: %v (id: %v)", task.f.Name, task.f.ID)
	}

	h := crc32.NewIEEE()
	_, err := io.Copy(h, r)
	if err != nil {
		return err
	}

	sum := h.Sum(nil)
	sumHex := hex.EncodeToString(sum)
	if sumHex != task.f.CRC32 {
		return fmt.Errorf("CRC32 check failed. got: %x want: %v", sumHex, task.f.CRC32)
	}

	return nil
}

// trimPath trims the given path.
// E.g. /usr/local/bin/foo becomes /u/l/b/foo.
func trimPath(p string) string {
	if len(p) < 60 {
		return p
	}
	p = filepath.Clean(p)
	parts := strings.Split(p, string(filepath.Separator))
	for i, part := range parts {
		if part == "" {
			parts[i] = string(filepath.Separator)
			continue
		}
		// skip last element
		if i == len(parts)-1 {
			continue
		}
		parts[i] = string(part[0])
	}

	return filepath.Join(parts...)
}
