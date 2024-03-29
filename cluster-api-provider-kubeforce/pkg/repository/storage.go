/*
Copyright 2022 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package repository

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

// NewStorage creates a new file storage.
func NewStorage(log logr.Logger, basePath string) *Storage {
	return &Storage{
		log:        log,
		keyedMutex: &keyedMutex{},
		basePath:   basePath,
	}
}

// Storage is a file cache for storing downloaded files.
type Storage struct {
	log        logr.Logger
	basePath   string
	keyedMutex *keyedMutex
}

type downloader func(ctx context.Context, w io.Writer) error

// GetHTTPFileGetter return FileGetter for HTTPRepository.
func (s *Storage) GetHTTPFileGetter(r infrav1.HTTPRepository) FileGetter {
	return NewHTTPFileGetter(s, r)
}

func existsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, errors.Wrapf(err, "unable to get file info %s", path)
	}
	if info.IsDir() {
		return false, errors.Errorf("found directory %s", path)
	}
	return true, nil
}

func (s *Storage) remove(relativePath string) error {
	return os.RemoveAll(path.Join(s.basePath, relativePath))
}

func (s *Storage) getFile(ctx context.Context, relativePath string, download downloader) (_ string, err error) {
	unlock := s.keyedMutex.Lock(relativePath)
	defer unlock()

	fullPath := path.Join(s.basePath, relativePath)

	ok, err := existsFile(fullPath)
	if err != nil {
		return "", err
	}
	if ok {
		return fullPath, nil
	}

	fullTmpPath := fullPath + ".tmp"
	ok, err = existsFile(fullTmpPath)
	if err != nil {
		return "", err
	}
	if ok {
		return "", errors.Errorf("temporary file found %q", fullTmpPath)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", err
	}
	f, err := os.OpenFile(filepath.Clean(fullTmpPath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	defer os.Remove(fullTmpPath)

	s.log.Info("downloading", "file", fullTmpPath)
	err = download(ctx, f)
	if err != nil {
		return "", errors.WithStack(err)
	}
	_ = f.Close()

	if err := os.Rename(fullTmpPath, fullPath); err != nil {
		return "", errors.WithStack(err)
	}
	return fullPath, nil
}

type keyedMutex struct {
	mutexes sync.Map
}

func (m *keyedMutex) Lock(key string) func() {
	value, _ := m.mutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := value.(*sync.Mutex)
	mtx.Lock()
	return func() { mtx.Unlock() }
}
