package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type watchFile struct {
	fd   *os.File
	info os.FileInfo
}

type DirReader struct {
	watcher *fsnotify.Watcher
	files   map[string]*watchFile
	tail    bool
}

func NewReader() (*DirReader, error) {
	d := new(DirReader)
	return d, d.Init()
}

func (d *DirReader) Init() (err error) {
	d.files = make(map[string]*watchFile)
	d.watcher, err = fsnotify.NewWatcher()
	d.tail = true
	return
}

func (d *DirReader) addFile(name string) (err error) {
	if _, ok := d.files[name]; ok {
		// panic(fmt.Sprintf("tried to add file twice: %s", name))
		return
	}
	var f *os.File
	f, err = os.Open(name)
	if err != nil {
		return
	}
	w := &watchFile{}
	w.fd = f
	if w.info, err = w.fd.Stat(); err != nil {
		return
	}
	if d.tail {
		// seek EOF
		_, err = w.fd.Seek(0, 2)
	}
	d.files[name] = w
	return
}

func (d *DirReader) scanFiles(dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode.IsDir() && dir != path {
			return d.scanFiles(path)
		}
		if mode.IsRegular() {
			err = d.addFile(path)
		}
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func (d *DirReader) readFile(name string, buf []byte) (n int, err error) {
	r := d.files[name].fd
	if n, err = r.Read(buf); err == io.EOF {
		err = nil
	}
	return
}

func (d *DirReader) removeFile(name string) error {
	if err := d.files[name].fd.Close(); err != nil {
		return err
	}
	d.files[name] = nil
	return nil
}

func (d *DirReader) getFileInfo(name string) (fi os.FileInfo, err error) {
	f := d.files[name]
	if f == nil {
		err = d.addFile(name)
		if err != nil {
			return
		}
		f = d.files[name]
	}
	fi = f.info
	return
}

func (d *DirReader) Read(buf []byte) (n int, err error) {
	var fi os.FileInfo
	for {
		select {
		case event := <-d.watcher.Events:
			fi, err = d.getFileInfo(event.Name)
			if err != nil {
				return
			}

			switch mode := fi.Mode(); {
			case mode.IsRegular() && event.Op&fsnotify.Write == fsnotify.Write:
				return d.readFile(event.Name, buf)
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				if err = d.Remove(event.Name); err != nil {
					return
				}
			case event.Op&fsnotify.Create == fsnotify.Create:
				if err = d.Watch(event.Name); err != nil {
					return
				}
			}
		case err = <-d.watcher.Errors:
			return
		}
	}
}

// Close will release all the resources held by this DirReader.
// Init must be called again to use this instance again.
func (d *DirReader) Close() error {
	for _, f := range d.files {
		if err := f.fd.Close(); err != nil {
			return err
		}
	}
	d.files = nil
	return d.watcher.Close()
}

// Watch adds directory to list of monitored directories
func (d *DirReader) Watch(dir string) (err error) {
	if err = d.watcher.Add(dir); err != nil {
		return
	}
	return d.scanFiles(dir)
}

func (d *DirReader) Remove(name string) error {
	if err := d.removeFile(name); err != nil {
		return err
	}
	// TODO recursively
	return d.watcher.Remove(name)
}
