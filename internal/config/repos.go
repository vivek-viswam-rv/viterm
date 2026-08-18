package config

import "fmt"

const reposFile = "repos.json"

// RepoRecord registers a repository sessions can be created from.
type RepoRecord struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
	// LayoutText is the pane layout used for new sessions of this repo.
	LayoutText string `json:"layoutText"`
	// PullBeforeWorktree updates the default branch before branching a new
	// worktree.
	PullBeforeWorktree bool `json:"pullBeforeWorktree"`
}

// NewRepoRecord builds a record with a fresh ID and pulling enabled.
func NewRepoRecord(path, name, layoutText string) RepoRecord {
	return RepoRecord{
		ID:                 randomID(),
		Path:               path,
		Name:               name,
		LayoutText:         layoutText,
		PullBeforeWorktree: true,
	}
}

// Repos loads the registered repository list.
func (s *Store) Repos() ([]RepoRecord, error) {
	var repos []RepoRecord
	err := s.readJSON(reposFile, &repos)
	return repos, err
}

// SaveRepos persists the repository list.
func (s *Store) SaveRepos(repos []RepoRecord) error {
	return s.writeJSON(reposFile, repos)
}

// AddRepo appends a record, generating an ID when it has none.
func (s *Store) AddRepo(r RepoRecord) error {
	if r.ID == "" {
		r.ID = randomID()
	}
	repos, err := s.Repos()
	if err != nil {
		return err
	}
	return s.SaveRepos(append(repos, r))
}

// RemoveRepo deletes the record with the given ID.
func (s *Store) RemoveRepo(id string) error {
	repos, err := s.Repos()
	if err != nil {
		return err
	}
	out := repos[:0]
	for _, r := range repos {
		if r.ID != id {
			out = append(out, r)
		}
	}
	return s.SaveRepos(out)
}

// UpdateRepo replaces the record with the same ID.
func (s *Store) UpdateRepo(r RepoRecord) error {
	repos, err := s.Repos()
	if err != nil {
		return err
	}
	for i := range repos {
		if repos[i].ID == r.ID {
			repos[i] = r
			return s.SaveRepos(repos)
		}
	}
	return fmt.Errorf("repo %s not found", r.ID)
}

// FindRepo looks a record up by ID.
func (s *Store) FindRepo(id string) (RepoRecord, bool, error) {
	repos, err := s.Repos()
	if err != nil {
		return RepoRecord{}, false, err
	}
	for _, r := range repos {
		if r.ID == id {
			return r, true, nil
		}
	}
	return RepoRecord{}, false, nil
}
