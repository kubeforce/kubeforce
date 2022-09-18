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

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

var fileContent = `
This is a test text file.
`

func TestSuccessfulUpload(t *testing.T) {
	ctx := context.Background()
	g := NewGomegaWithT(t)
	t.Run("upload the text file", func(t *testing.T) {
		tempDir := t.TempDir()
		targetPath := filepath.Join(tempDir, "test.txt")
		err := k8sClientset.UploadData(ctx, targetPath, []byte(fileContent), nil)
		g.Expect(err).Should(Succeed())
		data, err := os.ReadFile(filepath.Clean(targetPath))
		g.Expect(err).Should(Succeed())
		g.Expect(string(data)).Should(Equal(fileContent))
	})
}
