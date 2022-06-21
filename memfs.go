package memfs

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FS struct {
	root *dir
}

func NewFS() *FS {
	return &FS{root: new(dir)}
}

// TODO write this and export it
func fromFS(sys fs.FS, root string) (*FS, error) {
	f := NewFS()
	err := fs.WalkDir(sys, root, func(path string, d fs.DirEntry, err error) error {
		panic("not implemented")
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (f *FS) Open(name string) (fs.File, error) {
	entry, err := f.root.get(name)
	if err != nil {
		return nil, err
	}
	switch e := entry.(type) {
	case *dir:
		return e, nil
	case *file:
		return &fileReader{
			fileInfo: e.info(),
			Reader:   bytes.NewReader(e.body),
		}, nil
	default:
		return nil, fs.ErrInvalid
	}
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	entry, err := f.root.get(name)
	if err != nil {
		return nil, err
	}
	d, ok := entry.(*dir)
	if !ok {
		return nil, fs.ErrPermission
	}
	entries := make([]fs.DirEntry, 0)
	for _, child := range d.children {
		entries = append(entries, child)
	}
	return entries, nil
}

func (f *FS) Sub(directory string) (fs.FS, error) {
	e, err := f.root.get(directory)
	if err != nil {
		return nil, err
	}
	d, ok := e.(*dir)
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &FS{root: d}, nil
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	e, err := f.root.get(name)
	if err != nil {
		return nil, err
	}
	return e.Info()
}

func (f *FS) ReadFile(name string) ([]byte, error) {
	e, err := f.root.get(name)
	if err != nil {
		return nil, err
	}
	file, ok := e.(*file)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return file.body, nil
}

func (f *FS) Mkdir(path string) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "mkdir", Path: path, Err: fs.ErrInvalid}
	}
	d, name := filepath.Split(path)
	return f.root.add(d, &dir{name: name})
}

var (
	_ fs.FS         = (*FS)(nil)
	_ fs.StatFS     = (*FS)(nil)
	_ fs.SubFS      = (*FS)(nil)
	_ fs.ReadDirFS  = (*FS)(nil)
	_ fs.ReadFileFS = (*FS)(nil)
)

type dir struct {
	mu       sync.RWMutex
	name     string
	children map[string]fs.DirEntry
}

func (d *dir) Name() string               { return d.name }
func (d *dir) IsDir() bool                { return true }
func (d *dir) Type() fs.FileMode          { return d.Mode().Type() }
func (d *dir) Mode() fs.FileMode          { return fs.ModeDir }
func (d *dir) Sys() any                   { return &FS{} }
func (d *dir) ModTime() time.Time         { return time.Time{} }
func (d *dir) Size() int64                { return 0 }
func (d *dir) Info() (fs.FileInfo, error) { return d, nil }

func (d *dir) ReadDir(n int) ([]fs.DirEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if n <= 0 {
		n = len(d.children)
	}
	entries := make([]fs.DirEntry, n)
	i := 0
	for _, child := range d.children {
		if i >= n {
			break
		}
		entries[i] = child
		i++
	}
	return entries, nil
}

func (d *dir) Read([]byte) (int, error)   { return 0, fs.ErrPermission }
func (d *dir) Close() error               { return nil }
func (d *dir) Stat() (fs.FileInfo, error) { return d.Info() }

var _ fs.File = (*dir)(nil)

func (d *dir) get(name string) (fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	var (
		path    = split(name)
		end     = len(path) - 1
		current = d
	)
	for i, p := range path {
		current.mu.RLock()
		e, ok := current.children[p]
		if !ok {
			current.mu.RUnlock()
			return nil, &fs.PathError{
				Op:   "open",
				Path: name,
				Err:  fs.ErrNotExist,
			}
		}
		current.mu.RUnlock()
		if i == end {
			return e, nil
		}
		switch child := e.(type) {
		case *file:
			return nil, fs.ErrNotExist
		case *dir:
			current = child
		}
	}
	return nil, fs.ErrInvalid
}

func (d *dir) add(name string, e fs.DirEntry) error {
	var (
		path    = split(name)
		current = d
	)
	for _, p := range path {
		if p == "" || p == "/" {
			continue
		}
		current.mu.RLock()
		en, ok := current.children[p]
		if !ok {
			current.mu.RUnlock()
			return fs.ErrNotExist
		}
		current.mu.RUnlock()
		dir, ok := en.(*dir)
		if !ok {
			return fs.ErrExist
		}
		current = dir
	}
	current.mu.Lock()
	if current.children == nil {
		current.children = make(map[string]fs.DirEntry)
	}
	current.children[e.Name()] = e
	current.mu.Unlock()
	return nil
}

func (d *dir) mkdirAll(name string, entryDir *dir) error {
	var (
		path    = split(name)
		current = d
	)
	for _, p := range path {
		current.mu.Lock()
		en, ok := current.children[p]
		if !ok {
			if current.children == nil {
				current.children = make(map[string]fs.DirEntry)
			}
			dir := &dir{name: p}
			current.children[p] = dir
			current.mu.Unlock()
			current = dir
		} else {
			current.mu.Unlock()
			current, ok = en.(*dir)
			if !ok {
				return fs.ErrExist
			}
		}
	}
	current.mu.Lock()
	if current.children == nil {
		current.children = make(map[string]fs.DirEntry)
	}
	current.children[entryDir.name] = entryDir
	current.mu.Unlock()
	return nil
}

func (d *dir) remove(name string) error {
	var (
		path    = split(name)
		end     = len(path) - 1
		current = d
	)
	for i, p := range path {
		if i >= end {
			break
		}
		current.mu.RLock()
		e, ok := current.children[p]
		if !ok {
			current.mu.RUnlock()
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
		current.mu.RUnlock()
		current, ok = e.(*dir)
		if !ok {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
		}
	}
	n := path[end]
	current.mu.Lock()
	if _, ok := current.children[n]; !ok {
		return fs.ErrNotExist
	}
	delete(current.children, n)
	current.mu.Unlock()
	return nil
}

func split(path string) []string {
	paths := strings.Split(path, string(filepath.Separator))
	return paths
}

type file struct {
	body []byte
	fileInfo
}

func (f *file) Size() int64 { return int64(len(f.body)) }

func (f *file) info() *fileInfo {
	return &fileInfo{name: f.name, size: f.Size()}
}

type fileInfo struct {
	name string
	size int64
}

func (fi *fileInfo) Stat() (fs.FileInfo, error) { return fi.Info() }
func (fi *fileInfo) Close() error               { return nil }
func (fi *fileInfo) Name() string               { return fi.name }
func (fi *fileInfo) Size() int64                { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode          { return fs.FileMode(0) }
func (fi *fileInfo) ModTime() time.Time         { return time.Time{} }
func (fi *fileInfo) IsDir() bool                { return false }
func (fi *fileInfo) Sys() any                   { return &FS{} }
func (fi *fileInfo) Info() (fs.FileInfo, error) { return fi, nil }
func (fi *fileInfo) Type() fs.FileMode          { return fi.Mode().Type() }

type fileReader struct {
	*fileInfo
	*bytes.Reader
}
