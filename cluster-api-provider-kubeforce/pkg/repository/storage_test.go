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
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ FileGetter = &fakeFileGetter{}

type fakeFileGetter struct {
	s               *Storage
	content         string
	duration        time.Duration
	downloadCounter int
}

func (g *fakeFileGetter) RemoveCache() error {
	return errors.New("implement me")
}

func (g *fakeFileGetter) GetFile(ctx context.Context, relativePath string) (*File, error) {
	filePath, err := g.s.getFile(ctx, relativePath, g.download())
	if err != nil {
		return nil, err
	}
	return &File{
		Path: filePath,
	}, nil
}

func (g *fakeFileGetter) download() downloader {
	return func(ctx context.Context, w io.Writer) error {
		g.downloadCounter++
		time.Sleep(g.duration)
		_, err := w.Write([]byte(g.content))
		return err
	}
}

func newFakeFileGetter(s *Storage, content string, d time.Duration) *fakeFileGetter {
	return &fakeFileGetter{
		s:        s,
		content:  content,
		duration: d,
	}
}

func TestStorage_getFile_concurrently(t *testing.T) {
	g := NewWithT(t)
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "storage")
	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	logf.SetLogger(logger)
	storage := NewStorage(logger, basePath)
	contentTest := "test1"
	fileGetter := newFakeFileGetter(storage, contentTest, time.Second)
	wg := sync.WaitGroup{}
	wg.Add(2)
	var fileContent1 []byte
	var fileContent2 []byte
	go func() {
		f, err := fileGetter.GetFile(context.Background(), "file1.txt")
		g.Expect(err).ShouldNot(HaveOccurred())
		fileContent1, err = os.ReadFile(f.Path)
		g.Expect(err).ShouldNot(HaveOccurred())
		wg.Done()
	}()
	go func() {
		f, err := fileGetter.GetFile(context.Background(), "file1.txt")
		g.Expect(err).ShouldNot(HaveOccurred())
		fileContent2, err = os.ReadFile(f.Path)
		g.Expect(err).ShouldNot(HaveOccurred())
		wg.Done()
	}()
	wg.Wait()
	g.Expect(fileContent1).Should(Equal(fileContent2))
	g.Expect(fileGetter.downloadCounter).Should(Equal(1))
}
