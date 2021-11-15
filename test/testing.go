package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TmpFile struct {
	filename string
	file     *os.File
}

func NewTmpFile(t *testing.T) *TmpFile {
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	return &TmpFile{
		filename: f.Name(),
		file:     f,
	}
}

func (s *TmpFile) Name() string        { return s.filename }
func (s *TmpFile) File() *os.File      { return s.file }
func (s *TmpFile) Close(t *testing.T)  { assert.Nil(t, s.file.Close()) }
func (s *TmpFile) Remove(t *testing.T) { assert.Nil(t, os.Remove(s.file.Name())) }

type TmpDir struct {
	dir string
}

func NewTmpDir(t *testing.T) *TmpDir {
	dir, err := os.MkdirTemp("", "")
	assert.Nil(t, err)
	return &TmpDir{
		dir: dir,
	}
}

func (s *TmpDir) Dir() string             { return s.dir }
func (s *TmpDir) Path(name string) string { return filepath.Join(s.dir, name) }
func (s *TmpDir) Remove(t *testing.T)     { assert.Nil(t, os.RemoveAll(s.dir)) }
