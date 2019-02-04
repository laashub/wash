package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/puppetlabs/wash/log"
)

var slow = false

// DefaultTimeout is a default cache timeout.
const DefaultTimeout = 10 * time.Second

// Init sets up plugin core configuration on startup.
func Init(_slow bool) {
	slow = _slow
}

// ==== Plugin registry (FS) ====

// Root presents the root of the filesystem.
func (f *FS) Root() (fs.Node, error) {
	log.Printf("Entering root of filesystem")
	return &Dir{f}, nil
}

// Find the named item or return nil.
func (f *FS) Find(_ context.Context, name string) (Node, error) {
	if client, ok := f.Plugins[name]; ok {
		return &Dir{client}, nil
	}
	return nil, ENOENT
}

// List all clients as directories.
func (f *FS) List(_ context.Context) ([]Node, error) {
	keys := make([]Node, 0, len(f.Plugins))
	for _, v := range f.Plugins {
		keys = append(keys, &Dir{v})
	}
	return keys, nil
}

// Name returns '/'.
func (f *FS) Name() string {
	return "/"
}

// Attr returns basic (zero) attributes for the root directory.
func (f *FS) Attr(ctx context.Context) (*Attributes, error) {
	// Only ever called with "/". Return latest Mtime of all clients.
	var latest time.Time
	for _, v := range f.Plugins {
		attr, err := v.Attr(ctx)
		if err != nil {
			return nil, err
		}
		if attr.Mtime.After(latest) {
			latest = attr.Mtime
		}
	}
	return &Attributes{Mtime: latest, Valid: 100 * time.Millisecond}, nil
}

// Xattr returns an empty map.
func (f *FS) Xattr(ctx context.Context) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}

// ==== FUSE Directory Interface ====

// NewDir creates a new Dir object.
func NewDir(impl DirProtocol) *Dir {
	return &Dir{impl}
}

func (d *Dir) String() string {
	if v, ok := d.DirProtocol.(fmt.Stringer); ok {
		return v.String()
	}
	return d.Name()
}

// Attr returns the attributes of a directory.
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0550

	attr, err := d.DirProtocol.Attr(ctx)
	if err != nil {
		log.Printf("Error[Attr,%v]: %v", d, err)
	}
	a.Mtime = attr.Mtime
	a.Size = attr.Size
	a.Valid = attr.Valid
	log.Printf("Attr of dir %v: %v, %v", d, a.Mtime, a.Size)
	return err
}

// Listxattr lists extended attributes for the resource.
func (d *Dir) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	xattrs, err := d.Xattr(ctx)
	if err != nil {
		log.Printf("Error[Listxattr,%v]: %v", d, err)
		return err
	}

	for k := range xattrs {
		resp.Append(k)
	}
	log.Printf("Listxattr %v", d)
	return nil
}

// Getxattr gets extended attributes for the resource.
func (d *Dir) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	if req.Name == "com.apple.FinderInfo" {
		return nil
	}

	xattrs, err := d.Xattr(ctx)
	if err != nil {
		log.Printf("Error[Getxattr,%v,%v]: %v", d, req.Name, err)
		return err
	}

	resp.Xattr = xattrs[req.Name]
	log.Printf("Getxattr %v", d)
	return nil
}

func prefetch(entry fs.Node) {
	if slow {
		return
	}

	switch v := entry.(type) {
	case *Dir:
		go func() { v.List(context.Background()) }()
	case *File:
		// TODO: This can be pretty expensive. Probably better to move it to individual implementations
		// where they can choose to do this if Attr is requested.
		go func() {
			buf, err := v.FileProtocol.Open(context.Background())
			if closer, ok := buf.(io.Closer); err == nil && ok {
				go func() {
					time.Sleep(DefaultTimeout)
					closer.Close()
				}()
			}
		}()
	default:
		log.Debugf("Not sure how to prefetch for %v", v)
	}
}

// Lookup searches a directory for children.
func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	entry, err := d.Find(ctx, req.Name)
	if err == nil {
		log.Printf("Found %v in %v", req.Name, d)
		prefetch(entry)
	} else {
		log.Printf("%v not found in %v", req.Name, d)
	}
	return entry, err
}

// ReadDirAll lists all children of the directory.
func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	entries, err := d.List(ctx)
	if err != nil {
		log.Printf("Error[List,%v]: %v", d, err)
		return nil, err
	}

	log.Printf("Listed %v in %v", len(entries), d)

	res := make([]fuse.Dirent, len(entries))
	for i, entry := range entries {
		var de fuse.Dirent
		switch v := entry.(type) {
		case *Dir:
			de.Name = v.Name()
			de.Type = fuse.DT_Dir
		case *File:
			de.Name = v.Name()
		}
		res[i] = de
	}
	return res, nil
}

// ==== FUSE File Interface ====

// NewFile creates a new Dir object.
func NewFile(impl FileProtocol) *File {
	return &File{impl}
}

func (f *File) String() string {
	if v, ok := f.FileProtocol.(fmt.Stringer); ok {
		return v.String()
	}
	return f.Name()
}

// Attr returns the attributes of a file.
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0440

	attr, err := f.FileProtocol.Attr(ctx)
	if err != nil {
		log.Printf("Error[Attr,%v]: %v", f, err)
	}
	a.Mtime = attr.Mtime
	a.Size = attr.Size
	a.Valid = attr.Valid
	log.Printf("Attr of file %v: %v, %s", f, a.Size, a.Mtime)
	return err
}

// Listxattr lists extended attributes for the resource.
func (f *File) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	xattrs, err := f.Xattr(ctx)
	if err != nil {
		log.Printf("Error[Listxattr,%v]: %v", f, err)
		return err
	}

	for k := range xattrs {
		resp.Append(k)
	}
	log.Printf("Listxattr %v", f)
	return nil
}

// Getxattr gets extended attributes for the resource.
func (f *File) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	if req.Name == "com.apple.FinderInfo" {
		return nil
	}

	xattrs, err := f.Xattr(ctx)
	if err != nil {
		log.Printf("Error[Getxattr,%v,%v]: %v", f, req.Name, err)
		return err
	}

	resp.Xattr = xattrs[req.Name]
	log.Printf("Getxattr %v", f)
	return nil
}

// Open a file for reading.
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// Initiate content request and return a channel providing the results.
	log.Printf("Opening %v", f)
	r, err := f.FileProtocol.Open(ctx)
	if err != nil {
		log.Printf("Error[Open,%v]: %v", f, err)
		return nil, err
	}
	log.Printf("Opened %v", f)
	return &FileHandle{r: r}, nil
}

// Release closes the open file.
func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	if closer, ok := fh.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Read fills a buffer with the requested amount of data from the file.
func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	buf := make([]byte, req.Size)
	n, err := fh.r.ReadAt(buf, req.Offset)
	if err == io.EOF {
		err = nil
	}
	log.Printf("Read %v/%v bytes starting at %v: %v", n, req.Size, req.Offset, err)
	resp.Data = buf[:n]
	return err
}
