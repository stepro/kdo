package filesync

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type tarchive struct {
	root  string
	sync  [][2]string
	buf   bytes.Buffer
	tw    *tar.Writer
	items []string
	index int
}

func newTarchive(root string, sync [][2]string, items ...string) io.Reader {
	a := &tarchive{
		root: root,
		sync: sync,
	}
	a.tw = tar.NewWriter(&a.buf)
	a.items = items
	a.index = -1
	return a
}

func (a *tarchive) next() error {
	var name string
	var info os.FileInfo
	var err error
	for {
		a.index++
		if a.index > len(a.items) {
			return nil
		} else if a.index == len(a.items) {
			return a.tw.Close()
		}
		name = filepath.Join(a.root, a.items[a.index])
		info, err = os.Lstat(name)
		if err != nil {
			return err
		}
		im := info.Mode()
		if !im.IsRegular() && !im.IsDir() {
			// Skip unsupported type
			continue
		}
		break
	}

	path := a.items[a.index]
	for _, rule := range a.sync {
		if rule[0] != "" && !strings.HasPrefix(path, rule[0]+"/") {
			continue
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rule[1] + "/" + path
		if err = a.tw.WriteHeader(hdr); err != nil {
			return err
		} else if !info.Mode().IsDir() {
			if _, err = io.Copy(a.tw, file); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *tarchive) Read(p []byte) (n int, err error) {
	if a.buf.Len() == 0 {
		err = a.next()
		if err != nil {
			return 0, err
		}
	}

	if a.buf.Len() > 0 {
		n, err = a.buf.Read(p)
		return
	}

	return 0, io.EOF
}
