package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

// NewHTTPFileGetter creates a FileRepository for HTTPRepository
func NewHTTPFileGetter(s *Storage, r infrav1.HTTPRepository) FileRepository {
	return &HTTPFileRepository{
		repository: r,
		storage:    s,
	}
}

type HTTPFileRepository struct {
	repository infrav1.HTTPRepository
	storage    *Storage
}

func convertURLToFilesystemPath(url string) string {
	result := strings.ReplaceAll(url, "://", "/")
	result = strings.ReplaceAll(result, ":", "_")
	result = strings.ReplaceAll(result, "&", "_")
	result = strings.ReplaceAll(result, "|", "_")
	result = strings.ReplaceAll(result, ">", "_")
	result = strings.ReplaceAll(result, "<", "_")
	return result
}

func (g *HTTPFileRepository) GetFile(ctx context.Context, relativePath string) (*File, error) {
	parsedURL, err := url.Parse(g.repository.Spec.URL)
	if err != nil {
		return nil, err
	}
	parsedURL.Path = path.Join(parsedURL.Path, relativePath)

	fileURL := parsedURL.String()
	relativeFSPath := path.Join(g.repository.Namespace, g.repository.Name, convertURLToFilesystemPath(fileURL))
	fullFilePath, err := g.storage.getFile(ctx, relativeFSPath, g.download(fileURL))
	if err != nil {
		return nil, err
	}
	return &File{
		Path: fullFilePath,
	}, nil
}

func (g *HTTPFileRepository) RemoveCache() error {
	relativeFSPath := path.Join(g.repository.Namespace, g.repository.Name)
	return g.storage.remove(relativeFSPath)
}

func (g *HTTPFileRepository) download(url string) downloader {
	return func(ctx context.Context, w io.Writer) error {
		ctx, cancelFunc := context.WithTimeout(ctx, g.repository.Spec.Timeout.Duration)
		defer cancelFunc()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("unable to create request(GET): %q", url)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("unable to download file: %q", url)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %q url: %q", resp.Status, url)
		}
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			return err
		}
		return nil
	}
}
