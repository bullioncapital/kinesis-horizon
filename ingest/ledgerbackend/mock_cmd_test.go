package ledgerbackend

import (
	"io"
	"os"

	"github.com/stretchr/testify/mock"
)

type mockCmd struct {
	mock.Mock
}

func (m *mockCmd) Output() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCmd) Wait() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) setDir(dir string) {
	m.Called(dir)
}

func (m *mockCmd) setStdout(stdout io.Writer) {
	m.Called(stdout)
}

func (m *mockCmd) getStdout() io.Writer {
	args := m.Called()
	return args.Get(0).(io.Writer)
}

func (m *mockCmd) setStderr(stderr io.Writer) {
	m.Called(stderr)
}

func (m *mockCmd) getStderr() io.Writer {
	args := m.Called()
	return args.Get(0).(io.Writer)
}

func (m *mockCmd) getProcess() *os.Process {
	args := m.Called()
	return args.Get(0).(*os.Process)
}

func (m *mockCmd) setExtraFiles(files []*os.File) {
	m.Called(files)
}

func simpleCommandMock() *mockCmd {
	_, writer := io.Pipe()
	cmdMock := &mockCmd{}
	cmdMock.On("setDir", mock.Anything)
	cmdMock.On("setStdout", mock.Anything)
	cmdMock.On("getStdout").Return(writer)
	cmdMock.On("setStderr", mock.Anything)
	cmdMock.On("getStderr").Return(writer)
	cmdMock.On("getProcess").Return(&os.Process{})
	cmdMock.On("setExtraFiles", mock.Anything)
	cmdMock.On("Start").Return(nil)
	return cmdMock
}
