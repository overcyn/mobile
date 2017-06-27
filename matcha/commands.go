package matcha

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Creates a new temporary directory. Don't forget to remove.
func NewTmpDir(f *Flags, path string) (string, error) {
	// Make $GOPATH/pkg/work
	tmpdir := ""
	if f.ShouldRun() {
		var err error
		tmpdir, err = ioutil.TempDir(path, "gomobile-work-")
		if err != nil {
			return "", err
		}
	} else {
		if path == "" {
			tmpdir = "$WORK"
		} else {
			tmpdir = filepath.Join(path, "work")
		}
	}
	if f.ShouldPrint() {
		fmt.Fprintln(os.Stderr, "WORK="+tmpdir)
	}
	return tmpdir, nil
}

func RemoveAll(f *Flags, path string) error {
	if f.ShouldPrint() {
		Printcmd(`rm -r -f "%s"`, path)
	}
	if f.ShouldRun() {
		return os.RemoveAll(path)
	}
	return nil
}

func WriteFile(flags *Flags, filename string, generate func(io.Writer) error) error {
	if err := Mkdir(flags, filepath.Dir(filename)); err != nil {
		return err
	}
	if flags.ShouldPrint() {
		fmt.Fprintf(os.Stderr, "write %s\n", filename)
	}
	if flags.ShouldRun() {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		return generate(f)
	}
	return generate(ioutil.Discard)
}

func ReadFile(flags *Flags, filename string) ([]byte, error) {
	if flags.ShouldPrint() {
		fmt.Fprintf(os.Stderr, "read %s\n", filename)
	}
	if flags.ShouldRun() {
		return ioutil.ReadFile(filename)
	}
	return []byte{}, nil
}

func CopyFile(f *Flags, dst, src string) error {
	if f.ShouldPrint() {
		Printcmd("cp %s %s", src, dst)
	}
	return WriteFile(f, dst, func(w io.Writer) error {
		if f.ShouldRun() {
			f, err := os.Open(src)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(w, f); err != nil {
				return fmt.Errorf("cp %s %s failed: %v", src, dst, err)
			}
		}
		return nil
	})
}

func Mkdir(flags *Flags, dir string) error {
	if flags.ShouldPrint() {
		Printcmd("mkdir -p %s", dir)
	}
	if flags.ShouldRun() {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func Symlink(flags *Flags, src, dst string) error {
	if flags.ShouldPrint() {
		Printcmd("ln -s %s %s", src, dst)
	}
	if flags.ShouldRun() {
		// if goos == "windows" {
		//  return doCopyAll(dst, src)
		// }
		return os.Symlink(src, dst)
	}
	return nil
}
