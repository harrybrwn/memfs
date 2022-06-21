package memfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// I just don't want to keep typing it out
var join = filepath.Join

func Test(t *testing.T) {
}

func TestNewFS(t *testing.T) {
	f := NewFS()
	if f == nil {
		t.Fatal("new FS is nil")
	}
	if f.root == nil {
		t.Error("nil root dir")
	}
}

func TestFS_Open(t *testing.T) {
}

func TestFS_ReadDir(t *testing.T) {}

func TestFS_Sub(t *testing.T) {}

func TestFS_Stat(t *testing.T) {}

func TestFS_ReadFile(t *testing.T) {}

func TestDir(t *testing.T) {
	d := dir{}
	if !d.IsDir() {
		t.Error("a dir should know that it is a directory")
	}
	if d.Mode()&fs.ModeDir == 0 {
		t.Error("directories should have a directory file mode")
	}
	if d.Type()&fs.ModeDir == 0 {
		t.Error("directories should have a directory file mode")
	}
	if _, ok := d.Sys().(*FS); !ok {
		t.Error("dir.Sys should be an in memory filesystem")
	}
	if !d.ModTime().Equal(time.Time{}) {
		t.Error("dir.ModTime should have empty time")
	}
	if d.Size() != 0 {
		t.Error("dir.Size should be 0")
	}
	info, err := d.Info()
	if err != nil {
		t.Error(err)
	}
	if di, ok := info.(*dir); !ok {
		t.Error("dir.Info should return a *dir")
		if di.name != d.name {
			t.Error("info should have the same name as dir")
		}
	}
	if _, err = d.Stat(); err != nil {
		t.Fatal(err)
	}
	n, err := d.Read(nil)
	if n != 0 {
		t.Error("should read zero bytes from a dir")
	}
	if err != fs.ErrPermission {
		t.Error("expected fs.ErrPermission when reading from a dir")
	}
	if err := d.Close(); err != nil {
		t.Error(err)
	}
}

func newFile(name string, body []byte) *file {
	return &file{
		fileInfo: fileInfo{name: name, size: int64(len(body))},
		body:     body,
	}
}

type entry = fs.DirEntry

func TestDir_get(t *testing.T) {
	d := dir{children: map[string]entry{
		"one": &dir{
			name: "one",
			children: map[string]entry{
				"inner": &dir{
					name: "inner",
					children: map[string]entry{
						// "file.txt": &file{name: "file.txt"},
						"file.txt": newFile("file.txt", nil),
					},
				},
			},
		},
		// "two.txt": &file{name: "two.txt"},
		"two.txt": newFile("two.txt", nil),
	}}
	_, err := d.get("one///")
	if err == nil {
		t.Error("expected error: 'one///' is an invalid path")
	}
	e, err := d.get("one")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name() != "one" {
		t.Fatal("expect different directory name")
	}
	e, err = d.get(join("one", "inner", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name() != "file.txt" {
		t.Fatal("expected correct file name")
	}
	f, ok := e.(*file)
	if !ok {
		t.Fatalf("expected entry to be a file, got %T", e)
	}
	if f.name != e.Name() {
		t.Fatal("file.name does not match entry.Name")
	}
	e, err = d.get(join("one", "inner", "file.txt", "folder"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected error fs.ErrNotExist")
	}
	if !os.IsNotExist(err) {
		t.Error("expected error fs.ErrNotExist")
	}
}

func TestDir_add(t *testing.T) {
	b := &dir{name: "b", children: map[string]entry{}}
	d := dir{children: map[string]entry{
		"a": &dir{name: "a", children: map[string]entry{
			"b": b,
		}},
	}}

	err := d.add(join("a", "b"), newFile("f.txt", nil))
	if err != nil {
		t.Fatal(err)
	}
	e, exists := b.children["f.txt"]
	if !exists {
		t.Fatal("added file should exist")
	}
	entry, err := d.get(join("a", "b", "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if e != entry {
		t.Error("dir.get should return the same pointer that was just added")
	}

	err = d.add(join("a", "b"), &dir{name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	e, exists = b.children["x"]
	if !exists {
		t.Fatal("added directory should exist")
	}
	entry, err = d.get(join("a", "b", "x"))
	if err != nil {
		t.Fatal(err)
	}
	if e != entry {
		t.Error("dir.get should return the same pointer that was just added")
	}

	err = d.add("", &dir{name: "1"})
	if err != nil {
		t.Fatal(err)
	}
	err = d.add(join("a", "a-dir-that-has-no-been-added", "some-path"), newFile("test.txt", nil))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected fs.ErrNotExist from adding to folders that don't exist")
	}
	err = d.add(join("a", "b", "f.txt"), &dir{name: "butts"})
	if !errors.Is(err, fs.ErrExist) {
		t.Error("expected fs.ErrInvalid when adding a directory to a file")
	}
}

func TestDir_ReadDir(t *testing.T) {}

func TestDir_mkdirAll(t *testing.T) {
	var d dir
	err := d.mkdirAll("one/two/three/four", &dir{name: "five"})
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.get("one/two/three/four/five")
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsDir() {
		t.Fatal("should be a directory")
	}

	err = d.mkdirAll("one/two/red-fish/blue", &dir{name: "potato"})
	if err != nil {
		t.Fatal(err)
	}
	e, err = d.get("one/two/red-fish/blue/potato")
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsDir() {
		t.Fatal("should be a directory")
	}
}

func TestDir_remove(t *testing.T) {
	b := &dir{name: "b", children: map[string]entry{
		"x": newFile("x", nil),
	}}
	d := dir{children: map[string]entry{
		"a": &dir{name: "a", children: map[string]entry{
			"b": b,
		}},
	}}
	err := d.remove("a/b/x")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{
		"a/not-here/b/x",
		"a/not-here/b",
		"a/not-here/b/",
		"a/not-here",
	} {
		if !errors.Is(d.remove(f), fs.ErrNotExist) {
			t.Error("expected fs.ErrNotExist from removing files that don't exist")
		}
	}
}

// This test should break concurrent reads/writes if stuff is not protected.
func TestDir_concurrency(t *testing.T) {
	var (
		d     dir
		wg    sync.WaitGroup
		names = "abcdefghijklmnopqrstuvwxyz"
	)

	wg.Add(len(names))
	for _, c := range names {
		go func(c rune) {
			defer wg.Done()
			err := d.add("", &dir{name: string(c)})
			if err != nil {
				panic(err)
			}
			wg.Add(len(names) * 2)
			for _, c2 := range names {
				go func(c rune) {
					defer wg.Done()
					if _, err := d.get(string(c)); err != nil {
						panic(err)
					}
				}(c)
				go func(c2 rune) {
					defer wg.Done()
					err := d.add(string(c), &dir{name: string(c2)})
					if err != nil {
						panic(err)
					}
				}(c2)
			}
		}(c)
	}
	wg.Wait()

	l := len(names) * len(names)
	wg.Add(l * 2)
	for _, c1 := range names {
		for _, c2 := range names {
			p := join(string(c1), string(c2))
			go func(path string) {
				defer wg.Done()
				_, err := d.get(path)
				if err != nil {
					panic(err)
				}
			}(p)
			go func(path string) {
				defer wg.Done()
				err := d.add(path, newFile(path+".txt", nil))
				if err != nil {
					panic(err)
				}
			}(p)
		}
	}
	wg.Wait()
	if err := d.remove("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.get("a"); err == nil {
		t.Fatal("expected an error from getting deleted folder")
	}
}

func TestFile(t *testing.T) {
	f := newFile("test.txt", []byte("123"))
	if f.Name() != "test.txt" {
		t.Errorf("wrong name: got %q, want %q", f.Name(), "test.txt")
	}
	if f.Size() != 3 {
		t.Errorf("wrong size: get %d, want %d", f.Size(), 3)
	}
	if f.Mode() != fs.FileMode(0) {
		t.Error("wrong file mode")
	}
	if f.Type() != fs.FileMode(0) {
		t.Error("wrong file mode")
	}
	if !f.ModTime().Equal(time.Time{}) {
		t.Error("file.ModTime should have empty time")
	}
	if _, ok := f.Sys().(*FS); !ok {
		t.Error("dir.Sys should be an in memory filesystem")
	}
	if f.IsDir() {
		t.Error("file should be be a dir")
	}
	if f.Size() != f.info().Size() {
		t.Error("sizes should be the same")
	}
	info, err := f.Stat()
	if err != nil {
		t.Error(err)
	}
	if fi, ok := info.(*fileInfo); !ok {
		t.Error("file.Info should return a *fileInfo")
		if fi.name != f.name {
			t.Error("info should have the same name as dir")
		}
	}
	info, err = f.Info()
	if err != nil {
		t.Error(err)
	}
	if fi, ok := info.(*fileInfo); !ok {
		t.Error("file.Info should return a *fileInfo")
		if fi.name != f.name {
			t.Error("info should have the same name as dir")
		}
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestPathSplitting(t *testing.T) {
	// This test does not test package code, its just making sure im correct in
	// my assumptions about the behavior of the standard library.
	parts := strings.Split(
		join("one", "two"),
		string(filepath.Separator),
	)
	if len(parts) != 2 {
		t.Fatal("wrong split length")
	}
	if parts[0] != "one" {
		t.Fatal("expected \"one\"")
	}
	if parts[1] != "two" {
		t.Fatal("expected \"two\"")
	}
}
