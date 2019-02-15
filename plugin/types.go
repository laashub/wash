package plugin

import (
	"context"
	"io"
	"os"
	"time"
)

// ==== Wash Protocols and Resources ====

// Entry is a basic named resource type
type Entry interface{ Name() string }

// EntryBase implements Entry, making it easy to create new entries
type EntryBase struct{ name string }

// Name returns the entry's name.
func (e *EntryBase) Name() string { return e.name }

// Resource is an entry that has metadata.
type Resource interface {
	Entry
	Metadata(context.Context) (map[string]interface{}, error)
}

// Group is an entry that can list its contents, an array of entries.
// It will be represented as a directory in the wash filesystem.
type Group interface {
	Entry
	LS(context.Context) ([]Entry, error)
}

// Root is the root object of a plugin.
type Root interface {
	Group
	Init(name string) error
}

// Execable is an entry that can have a command run on it.
type Execable interface {
	Exec(context.Context, string) (io.Reader, error)
}

// File is an entry that specifies filesystem attributes.
type File interface {
	Entry
	Attr() Attributes
}

// Pipe is an entry that returns a stream of updates.
type Pipe interface {
	Stream(context.Context) (io.Reader, error)
}

// SizedReader returns a ReaderAt that can report its Size.
type SizedReader interface {
	io.ReaderAt
	Size() int64
}

// Readable is an entry that has a fixed amount of content we can read.
type Readable interface {
	Open(context.Context) (SizedReader, error)
}

// Writable is an entry that we can write new data to.
type Writable interface {
	Save(context.Context, io.Reader) error
}

// Attributes of resources.
type Attributes struct {
	Atime time.Time
	Mtime time.Time
	Ctime time.Time
	Mode  os.FileMode
	Size  uint64
	Valid time.Duration
}

// Protocols is the list of supported protocols for Wash
const (
	MetadataCommand = "metadata"
	ListCommand     = "list"
	ReadCommand     = "read"
)

// The Registry contains the core filesystem data.
// Plugins: maps plugin mount points to their implementations.
type Registry struct {
	Plugins map[string]Root
}
