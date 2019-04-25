package volume

import (
	"bytes"
	"context"
	"fmt"

	"github.com/puppetlabs/wash/activity"
	"github.com/puppetlabs/wash/plugin"
)

// FS presents a view of the filesystem of an Execable resource.
type FS struct {
	plugin.EntryBase
	executor plugin.Execable
}

// NewFS creates a new FS entry with the given name, using the supplied executor to satisfy volume
// operations.
func NewFS(name string, executor plugin.Execable) *FS {
	fs := &FS{EntryBase: plugin.NewEntry(name), executor: executor}
	// Caching handled in List.
	fs.DisableCachingFor(plugin.ListOp)
	return fs
}

// List will attempt to list the filesystem of an Execable resource (the executor). It will list
// a directory tree based on supplied configuration (defaulting to `/var/log` if not specified).
func (d *FS) List(ctx context.Context) ([]plugin.Entry, error) {
	return List(ctx, d, "")
}

func exec(ctx context.Context, executor plugin.Execable, cmdline []string) (*bytes.Buffer, error) {
	result, err := executor.Exec(ctx, cmdline[0], cmdline[1:], plugin.ExecOptions{})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var errs []error
	for chunk := range result.OutputCh {
		if chunk.Err != nil {
			errs = append(errs, chunk.Err)
		} else if chunk.StreamID == 0 {
			activity.Record(ctx, "Stdout: %v", chunk.Data)
			fmt.Fprint(&buf, chunk.Data)
		} else {
			activity.Record(ctx, "Stderr: %v", chunk.Data)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("exec errored: %v", errs)
	}

	exitcode, err := result.ExitCodeCB()
	if err != nil {
		return nil, err
	} else if exitcode != 0 {
		return nil, fmt.Errorf("exec exited non-zero")
	}
	return &buf, nil
}

// VolumeList satisfies the Interface required by List to enumerate files.
func (d *FS) VolumeList(ctx context.Context) (DirMap, error) {
	cmdline := StatCmd("/var/log")
	activity.Record(ctx, "Running %v on %v", cmdline, plugin.ID(d.executor))
	buf, err := exec(ctx, d.executor, cmdline)
	if err != nil {
		activity.Record(ctx, "Exec error running %v in VolumeList: %v", cmdline, err)
		return nil, err
	}
	activity.Record(ctx, "VolumeList complete")
	return StatParseAll(buf, "")
}

// VolumeOpen satisfies the Interface required by List to read file contents.
func (d *FS) VolumeOpen(ctx context.Context, path string) (plugin.SizedReader, error) {
	activity.Record(ctx, "Reading %v on %v", path, plugin.ID(d.executor))
	buf, err := exec(ctx, d.executor, []string{"cat", path})
	if err != nil {
		activity.Record(ctx, "Exec error running 'cat %v' in VolumeOpen: %v", path, err)
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
