package app

import (
	"github.com/htol/bopds/config"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/service"
)

type Server struct {
	storage     *repo.Repo
	service     *service.Service
	config      *config.Config
	libraryPath string
}

func NewServer(libraryPath string, storage *repo.Repo, cfg *config.Config) *Server {
	return &Server{
		storage:     storage,
		service:     service.New(storage),
		config:      cfg,
		libraryPath: libraryPath,
	}
}

func (s *Server) Close() error {
	if s.storage != nil {
		if err := s.storage.Close(); err != nil {
			return err
		}
	}
	return nil
}
