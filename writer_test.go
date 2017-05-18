package vc

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestWriter(t *testing.T) {
	DebugLogFunc = func(message string) {
		t.Log(message)
	}

	temp, err := ioutil.TempFile(os.TempDir(), "test")
	if err != nil {
		t.Skip(err)
	}
	name := temp.Name()
	if err = os.Remove(name); err != nil {
		t.Skip(err)
	}

	w := SafeOutputWriter(name, 0644)
	if i, err := os.Stat(name); err == nil {
		t.Fatalf("expected %s to not exist; but got %+v", name, i)
	}

	if _, err = w.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}

	if i, err := os.Stat(name); err != nil {
		t.Fatalf("expected %s to exist; but got %v", name, err)
	} else if m := i.Mode(); m != 0644 {
		t.Fatalf("expected %s to have mode %04o; but got %04o", name, 0644, m)
	}

	if err = os.Remove(name); err != nil {
		t.Fatal(err)
	}

	w = SafeOutputWriter(name, 0644)
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if i, err := os.Stat(name); err == nil {
		t.Fatalf("expected %s to not exist; but got %+v", name, i)
	}
}

func TestStdoutWriter(t *testing.T) {
	DebugLogFunc = func(message string) {
		t.Log(message)
	}

	org := os.Stdout
	defer func() {
		os.Stdout = org
	}()

	tmp, err := ioutil.TempFile(os.TempDir(), "stdout")
	if err != nil {
		t.Skip(err)
	}
	os.Stdout = tmp

	w := SafeOutputWriter("-", 0644)
	if _, err := w.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	os.Remove(tmp.Name())
}
