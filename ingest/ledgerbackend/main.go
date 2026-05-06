package ledgerbackend

import (
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
)

type isDir interface {
	IsDir() bool
}

type systemCaller interface {
	removeAll(path string) error
	writeFile(filename string, data []byte, perm fs.FileMode) error
	mkdirAll(path string, perm os.FileMode) error
	stat(name string) (isDir, error)
	command(name string, arg ...string) cmdI
}

type realSystemCaller struct{}

func (realSystemCaller) removeAll(path string) error {
	return os.RemoveAll(path)
}

func (realSystemCaller) writeFile(filename string, data []byte, perm fs.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

func (realSystemCaller) mkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (realSystemCaller) stat(name string) (isDir, error) {
	return os.Stat(name)
}

func (realSystemCaller) command(name string, arg ...string) cmdI {
	cmd := exec.Command(name, arg...)
	return &realCmd{cmd}
}

type cmdI interface {
	Output() ([]byte, error)
	Wait() error
	Start() error
	Run() error
	setDir(dir string)
	setStdout(stdout io.Writer)
	getStdout() io.Writer
	setStderr(stderr io.Writer)
	getStderr() io.Writer
	getProcess() *os.Process
	setExtraFiles([]*os.File)
}

type realCmd struct {
	*exec.Cmd
}

func (r *realCmd) setDir(dir string) {
	r.Cmd.Dir = dir
}

func (r *realCmd) setStdout(stdout io.Writer) {
	r.Cmd.Stdout = stdout
}

func (r *realCmd) getStdout() io.Writer {
	return r.Cmd.Stdout
}

func (r *realCmd) setStderr(stderr io.Writer) {
	r.Cmd.Stderr = stderr
}

func (r *realCmd) getStderr() io.Writer {
	return r.Cmd.Stderr
}

func (r *realCmd) getProcess() *os.Process {
	return r.Cmd.Process
}

func (r *realCmd) setExtraFiles(extraFiles []*os.File) {
	r.ExtraFiles = extraFiles
}
