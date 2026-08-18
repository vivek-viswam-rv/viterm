package config

const sessionsFile = "sessions.json"

// PRInfo caches the pull request last resolved for a session.
type PRInfo struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	State   string `json:"state"` // OPEN, MERGED, or CLOSED
	IsDraft bool   `json:"isDraft"`
}

// SessionRecord persists one workspace session, keyed by its worktree path.
type SessionRecord struct {
	WorktreePath string `json:"worktreePath"`
	RepoName     string `json:"repoName"`
	SessionName  string `json:"sessionName"`
	// Slug is the sanitized worktree directory and branch name.
	Slug       string `json:"slug"`
	LayoutText string `json:"layoutText"`
	// Attached sessions are reopened on launch; detached ones stay listed.
	Attached bool `json:"attached"`
	// ClaudeSessionID records the agent conversation to resume on reattach.
	ClaudeSessionID string  `json:"claudeSessionId,omitempty"`
	PR              *PRInfo `json:"pr,omitempty"`
}

// Sessions loads the session list.
func (s *Store) Sessions() ([]SessionRecord, error) {
	var sessions []SessionRecord
	err := s.readJSON(sessionsFile, &sessions)
	return sessions, err
}

// SaveSessions persists the session list.
func (s *Store) SaveSessions(sessions []SessionRecord) error {
	return s.writeJSON(sessionsFile, sessions)
}

// UpsertSession inserts or replaces the record for its worktree path.
func (s *Store) UpsertSession(rec SessionRecord) error {
	sessions, err := s.Sessions()
	if err != nil {
		return err
	}
	for i := range sessions {
		if sessions[i].WorktreePath == rec.WorktreePath {
			sessions[i] = rec
			return s.SaveSessions(sessions)
		}
	}
	return s.SaveSessions(append(sessions, rec))
}

// RemoveSession deletes the record for a worktree path.
func (s *Store) RemoveSession(worktreePath string) error {
	sessions, err := s.Sessions()
	if err != nil {
		return err
	}
	out := sessions[:0]
	for _, rec := range sessions {
		if rec.WorktreePath != worktreePath {
			out = append(out, rec)
		}
	}
	return s.SaveSessions(out)
}

// SetSessionAttached updates the attached flag for a worktree path.
func (s *Store) SetSessionAttached(worktreePath string, attached bool) error {
	return s.updateSession(worktreePath, func(rec *SessionRecord) {
		rec.Attached = attached
	})
}

// SetClaudeSessionID records the agent conversation ID for a worktree path.
func (s *Store) SetClaudeSessionID(worktreePath, id string) error {
	return s.updateSession(worktreePath, func(rec *SessionRecord) {
		rec.ClaudeSessionID = id
	})
}

// SetSessionPR caches the resolved pull request for a worktree path.
func (s *Store) SetSessionPR(worktreePath string, pr *PRInfo) error {
	return s.updateSession(worktreePath, func(rec *SessionRecord) {
		rec.PR = pr
	})
}

func (s *Store) updateSession(worktreePath string, fn func(*SessionRecord)) error {
	sessions, err := s.Sessions()
	if err != nil {
		return err
	}
	for i := range sessions {
		if sessions[i].WorktreePath == worktreePath {
			fn(&sessions[i])
			return s.SaveSessions(sessions)
		}
	}
	return nil
}
