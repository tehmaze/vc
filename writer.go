package vc

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

var (
	stdoutName = map[string]bool{
		"":            true,
		"-":           true,
		"/dev/stdout": true,
	}
	stderrName = map[string]bool{
		"/dev/stderr": true,
	}
)

// SafeOutputWriter implements a io.WriteCloser that uses a temporary
// file in the same directory as the target file to write to, and then move
// the temporary file to the final name after closing. If name is "" or "-",
// it is assumed the output is stdout and no tempfile will be used.
//
// The tempfile gets created on the first write to the returned Writer.
func SafeOutputWriter(name string, mode os.FileMode) io.WriteCloser {
	if stdoutName[name] {
		return os.Stdout
	} else if stderrName[name] {
		return os.Stderr
	}
	return &safeOutputWriter{
		name: name,
		mode: mode,
	}
}

type safeOutputWriter struct {
	name, temp string
	mode       os.FileMode
	mutex      sync.Mutex
	file       *os.File
}

func (w *safeOutputWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.file != nil {
		defer func() {
			w.file = nil
		}()
		return w.file.Close()
	}

	return nil
}

func (w *safeOutputWriter) Write(p []byte) (int, error) {
	if err := w.maybeOpenWriter(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *safeOutputWriter) maybeOpenWriter() (err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.file == nil {
		dir, base := filepath.Split(w.name)
		base = "." + base + "."

		if w.file, err = ioutil.TempFile(dir, base); err != nil {
			return
		}
		if err = w.file.Chmod(w.mode); err != nil {
			return
		}
		w.temp = w.file.Name()
	}

	return
}
