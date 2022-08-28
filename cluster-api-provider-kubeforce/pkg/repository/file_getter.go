package repository

import (
	"context"
)

type File struct {
	Path string
}

type FileRepository interface {
	GetFile(ctx context.Context, relativePath string) (*File, error)
	RemoveCache() error
}
