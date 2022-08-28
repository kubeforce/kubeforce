/*
Copyright 2019 The Kubernetes Authors.

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

package cloudinit

import (
	"bytes"
	"compress/gzip"
	"testing"

	. "github.com/onsi/gomega"
)

func TestFixContent(t *testing.T) {
	v := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gv, _ := gZipData([]byte(v))
	var useCases = []struct {
		name            string
		content         string
		encoding        string
		expectedContent string
		expectedError   bool
	}{
		{
			name:            "plain text",
			content:         "foobar",
			expectedContent: "foobar",
		},
		{
			name:            "base64 data",
			content:         "YWJjMTIzIT8kKiYoKSctPUB+",
			encoding:        "base64",
			expectedContent: "abc123!?$*&()'-=@~",
		},
		{
			name:            "gzip data",
			content:         string(gv),
			encoding:        "gzip",
			expectedContent: v,
		},
	}

	for _, rt := range useCases {
		t.Run(rt.name, func(t *testing.T) {
			g := NewWithT(t)

			encoding := fixEncoding(rt.encoding)
			c, err := fixContent(rt.content, encoding)
			if rt.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			g.Expect(rt.expectedContent).To(Equal(c))
		})
	}
}

func TestUnzipData(t *testing.T) {
	g := NewWithT(t)

	value := []byte("foobarbazquxfoobarbazquxfoobarbazquxfoobarbazquxfoobarbazquxfoobarbazquxfoobarbazquxfoobarbazquxfoobarbazqux")
	gvalue, _ := gZipData(value)
	dvalue, _ := gUnzipData(gvalue)
	g.Expect(value).To(Equal(dvalue))
}

func gZipData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)

	if _, err := gz.Write(data); err != nil {
		return nil, err
	}

	if err := gz.Flush(); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
