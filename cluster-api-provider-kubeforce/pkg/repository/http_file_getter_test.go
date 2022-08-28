package repository

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func testHTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/repository/v0.0.1/file1.txt", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testContent))
	})
	return mux
}

const (
	testName      = "test-name"
	testNamespace = "test-ns"
	testPath      = "/repository/v0.0.1/file1.txt"
	testContent   = "testContent"
)

func newHTTPRepo(url string) *infrav1.HTTPRepository {
	return &infrav1.HTTPRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: infrav1.HTTPRepositorySpec{
			URL: url,
		},
	}
}

func TestHTTPFileGetter_GetFile(t *testing.T) {
	server := httptest.NewServer(testHTTPHandler())
	defer server.Close()
	repo := newHTTPRepo(server.URL)
	g := NewWithT(t)
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "storage")
	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	logf.SetLogger(logger)
	storage := NewStorage(logger, basePath)
	fileGetter := storage.GetHTTPFileGetter(*repo)
	f, err := fileGetter.GetFile(context.Background(), testPath)
	g.Expect(err).ShouldNot(HaveOccurred())
	fileContent1, err := ioutil.ReadFile(f.Path)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(fileContent1).To(Equal([]byte(testContent)))
}
