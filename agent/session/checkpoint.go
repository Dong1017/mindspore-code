package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
)

const (
	checkpointsDirname = "checkpoints"
	messageIDPrefix    = "msg_"
	backupIDPrefix     = "bak_"
)

// ResumeStateRecord stores one restorable conversation state inside trajectory.jsonl.
type ResumeStateRecord struct {
	Type          string         `json:"type"`
	Sequence      int            `json:"sequence"`
	UpdatedAt     time.Time      `json:"updated_at"`
	SystemPrompt  string         `json:"system_prompt"`
	Messages      []llm.Message  `json:"messages,omitempty"`
	ProviderUsage *UsageSnapshot `json:"provider_usage,omitempty"`
}

// FileCheckpointRef points to one tracked file backup for a checkpointed turn.
type FileCheckpointRef struct {
	Path     string `json:"path"`
	BackupID string `json:"backup_id,omitempty"`
	Existed  bool   `json:"existed"`
	Mode     uint32 `json:"mode,omitempty"`
}

// CheckpointRecord stores one rewindable user turn marker.
type CheckpointRecord struct {
	Type           string              `json:"type"`
	Timestamp      time.Time           `json:"timestamp"`
	MessageID      string              `json:"message_id"`
	Preview        string              `json:"preview"`
	ResumeStateSeq int                 `json:"resume_state_seq"`
	Files          []FileCheckpointRef `json:"files,omitempty"`
}

// CheckpointSummary is the UI-facing shape used by rewind pickers.
type CheckpointSummary struct {
	MessageID      string
	Timestamp      time.Time
	Preview        string
	HasCodeRestore bool
}

type trajectoryEntry struct {
	kind       string
	message    MessageRecord
	resume     ResumeStateRecord
	checkpoint CheckpointRecord
}

func makeTrajectoryEntry(record any) trajectoryEntry {
	switch v := record.(type) {
	case MessageRecord:
		v.Arguments = append([]byte(nil), v.Arguments...)
		return trajectoryEntry{kind: v.Type, message: v}
	case ResumeStateRecord:
		v.Messages = cloneMessages(v.Messages)
		v.ProviderUsage = cloneUsageSnapshot(v.ProviderUsage)
		return trajectoryEntry{kind: v.Type, resume: v}
	case CheckpointRecord:
		v.Files = cloneFileRefs(v.Files)
		return trajectoryEntry{kind: v.Type, checkpoint: v}
	default:
		panic(fmt.Sprintf("unsupported trajectory record %T", record))
	}
}

func (e trajectoryEntry) record() any {
	switch e.kind {
	case recordTypeUser, recordTypeAssistant, recordTypeToolCall, recordTypeToolResult, recordTypeSkill, recordTypeCompact:
		record := e.message
		record.Arguments = append([]byte(nil), record.Arguments...)
		return record
	case recordTypeResumeState:
		record := e.resume
		record.Messages = cloneMessages(record.Messages)
		record.ProviderUsage = cloneUsageSnapshot(record.ProviderUsage)
		return record
	case recordTypeCheckpoint:
		record := e.checkpoint
		record.Files = cloneFileRefs(record.Files)
		return record
	default:
		panic(fmt.Sprintf("unsupported trajectory entry %q", e.kind))
	}
}

func (e trajectoryEntry) clone() trajectoryEntry {
	return makeTrajectoryEntry(e.record())
}

func cloneMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]llm.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = msg
		if len(msg.ToolCalls) > 0 {
			cloned[i].ToolCalls = make([]llm.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				cloned[i].ToolCalls[j] = tc
				cloned[i].ToolCalls[j].Function.Arguments = append([]byte(nil), tc.Function.Arguments...)
			}
		}
	}
	return cloned
}

func cloneFileRefs(refs []FileCheckpointRef) []FileCheckpointRef {
	if len(refs) == 0 {
		return nil
	}
	cloned := make([]FileCheckpointRef, len(refs))
	copy(cloned, refs)
	return cloned
}

func snapshotFromResumeState(sessionID, workDir string, record ResumeStateRecord) Snapshot {
	return Snapshot{
		SessionID:     sessionID,
		WorkDir:       workDir,
		SystemPrompt:  record.SystemPrompt,
		UpdatedAt:     record.UpdatedAt,
		Messages:      cloneMessages(record.Messages),
		ProviderUsage: cloneUsageSnapshot(record.ProviderUsage),
	}
}

func parseSequenceSuffix(value, prefix string) int {
	if !strings.HasPrefix(value, prefix) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (s *Session) applyTrajectoryEntryLocked(entry trajectoryEntry) {
	if s.checkpoints == nil {
		s.checkpoints = make(map[string]CheckpointRecord)
	}
	s.log = append(s.log, entry.clone())

	switch entry.kind {
	case recordTypeUser, recordTypeAssistant, recordTypeToolCall, recordTypeToolResult, recordTypeSkill, recordTypeCompact:
		record := entry.message
		s.records = append(s.records, record)
		if record.Timestamp.After(s.meta.UpdatedAt) {
			s.meta.UpdatedAt = record.Timestamp
		}
		if record.Type == recordTypeUser {
			if seq := parseSequenceSuffix(strings.TrimSpace(record.MessageID), messageIDPrefix); seq > s.nextMessageSeq {
				s.nextMessageSeq = seq
			}
		}
	case recordTypeResumeState:
		record := entry.resume
		s.resumeStates = append(s.resumeStates, record)
		if record.Sequence > s.nextResumeStateSeq {
			s.nextResumeStateSeq = record.Sequence
		}
		s.snapshot = snapshotFromResumeState(s.meta.SessionID, s.meta.WorkDir, record)
		s.meta.SystemPrompt = record.SystemPrompt
		if record.UpdatedAt.After(s.meta.UpdatedAt) {
			s.meta.UpdatedAt = record.UpdatedAt
		}
		s.bootstrapResumeState = false
	case recordTypeCheckpoint:
		record := entry.checkpoint
		messageID := strings.TrimSpace(record.MessageID)
		if messageID == "" {
			return
		}
		if _, exists := s.checkpoints[messageID]; !exists {
			s.checkpointOrder = append(s.checkpointOrder, messageID)
		}
		s.checkpoints[messageID] = record
		s.activeCheckpointMessageID = messageID
		if record.Timestamp.After(s.meta.UpdatedAt) {
			s.meta.UpdatedAt = record.Timestamp
		}
		for _, ref := range record.Files {
			if seq := parseSequenceSuffix(strings.TrimSpace(ref.BackupID), backupIDPrefix); seq > s.nextBackupSeq {
				s.nextBackupSeq = seq
			}
		}
	}
}

func (s *Session) appendTrajectoryEntryLocked(record any) error {
	entry := makeTrajectoryEntry(record)
	s.applyTrajectoryEntryLocked(entry)
	if !s.persisted {
		return nil
	}
	return s.writeRecordLocked(entry.record())
}

func (s *Session) latestResumeStateSeqLocked() int {
	if len(s.resumeStates) == 0 {
		return 0
	}
	return s.resumeStates[len(s.resumeStates)-1].Sequence
}

func (s *Session) ensureResumeStateBootstrapLocked() error {
	if s == nil || !s.bootstrapResumeState {
		return nil
	}

	record := ResumeStateRecord{
		Type:          recordTypeResumeState,
		Sequence:      s.nextResumeStateSeq + 1,
		UpdatedAt:     s.snapshot.UpdatedAt,
		SystemPrompt:  s.snapshot.SystemPrompt,
		Messages:      cloneMessages(s.snapshot.Messages),
		ProviderUsage: cloneUsageSnapshot(s.snapshot.ProviderUsage),
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now()
	}
	return s.appendTrajectoryEntryLocked(record)
}

func (s *Session) nextMessageIDLocked() string {
	s.nextMessageSeq++
	return fmt.Sprintf("%s%06d", messageIDPrefix, s.nextMessageSeq)
}

func (s *Session) nextBackupIDLocked() string {
	s.nextBackupSeq++
	return fmt.Sprintf("%s%06d", backupIDPrefix, s.nextBackupSeq)
}

// ListCheckpoints returns the current session's rewindable user checkpoints, newest first.
func (s *Session) ListCheckpoints() []CheckpointSummary {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]CheckpointSummary, 0, len(s.checkpointOrder))
	for i := len(s.checkpointOrder) - 1; i >= 0; i-- {
		messageID := s.checkpointOrder[i]
		record, ok := s.checkpoints[messageID]
		if !ok {
			continue
		}
		summaries = append(summaries, CheckpointSummary{
			MessageID:      record.MessageID,
			Timestamp:      record.Timestamp,
			Preview:        record.Preview,
			HasCodeRestore: len(record.Files) > 0,
		})
	}
	return summaries
}

func normalizeCheckpointPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." {
		return ""
	}
	return path
}

func sessionCheckpointsDir(trajectoryPath string) string {
	return filepath.Join(filepath.Dir(trajectoryPath), checkpointsDirname)
}

func checkpointBackupPath(trajectoryPath, backupID string) string {
	return filepath.Join(sessionCheckpointsDir(trajectoryPath), backupID)
}

// RecordFileMutation captures a turn-scoped backup before write/edit mutates a tracked file.
func (s *Session) RecordFileMutation(path, fullPath string) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path = normalizeCheckpointPath(path)
	if path == "" {
		return nil
	}
	messageID := strings.TrimSpace(s.activeCheckpointMessageID)
	if messageID == "" {
		return nil
	}
	record, ok := s.checkpoints[messageID]
	if !ok {
		return nil
	}
	for _, ref := range record.Files {
		if normalizeCheckpointPath(ref.Path) == path {
			return nil
		}
	}

	ref := FileCheckpointRef{
		Path: path,
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat file backup source %s: %w", path, err)
		}
		record.Files = append(record.Files, ref)
		return s.appendTrajectoryEntryLocked(record)
	}
	if info.IsDir() {
		return fmt.Errorf("cannot checkpoint directory %s", path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read file backup source %s: %w", path, err)
	}
	if err := os.MkdirAll(sessionCheckpointsDir(s.path), 0o755); err != nil {
		return fmt.Errorf("create session checkpoint dir: %w", err)
	}

	ref.Existed = true
	ref.Mode = uint32(info.Mode().Perm())
	ref.BackupID = s.nextBackupIDLocked()
	if err := os.WriteFile(checkpointBackupPath(s.path, ref.BackupID), data, 0o600); err != nil {
		return fmt.Errorf("write checkpoint backup %s: %w", path, err)
	}

	record.Files = append(record.Files, ref)
	return s.appendTrajectoryEntryLocked(record)
}

func (s *Session) checkpointRecordLocked(messageID string) (CheckpointRecord, bool) {
	if s == nil {
		return CheckpointRecord{}, false
	}
	record, ok := s.checkpoints[strings.TrimSpace(messageID)]
	return record, ok
}

func (s *Session) resumeStateForCheckpointLocked(record CheckpointRecord) (ResumeStateRecord, error) {
	for _, state := range s.resumeStates {
		if state.Sequence == record.ResumeStateSeq {
			return ResumeStateRecord{
				Type:          state.Type,
				Sequence:      state.Sequence,
				UpdatedAt:     state.UpdatedAt,
				SystemPrompt:  state.SystemPrompt,
				Messages:      cloneMessages(state.Messages),
				ProviderUsage: cloneUsageSnapshot(state.ProviderUsage),
			}, nil
		}
	}
	return ResumeStateRecord{}, fmt.Errorf("resume state %d not found", record.ResumeStateSeq)
}

// RestoreCheckpointContext returns the restorable conversation state from before the selected user message.
func (s *Session) RestoreCheckpointContext(messageID string) (string, []llm.Message, *UsageSnapshot, error) {
	if s == nil {
		return "", nil, nil, fmt.Errorf("session is nil")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.checkpointRecordLocked(messageID)
	if !ok {
		return "", nil, nil, fmt.Errorf("checkpoint %s not found", strings.TrimSpace(messageID))
	}
	state, err := s.resumeStateForCheckpointLocked(record)
	if err != nil {
		return "", nil, nil, err
	}
	return state.SystemPrompt, cloneMessages(state.Messages), cloneUsageSnapshot(state.ProviderUsage), nil
}

// RestoreCheckpointFiles restores tracked write/edit paths from before the selected user message.
func (s *Session) RestoreCheckpointFiles(messageID string) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}

	s.mu.RLock()
	record, ok := s.checkpointRecordLocked(messageID)
	workDir := s.meta.WorkDir
	path := s.path
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("checkpoint %s not found", strings.TrimSpace(messageID))
	}

	for _, ref := range record.Files {
		targetPath := filepath.Join(workDir, filepath.FromSlash(ref.Path))
		if !ref.Existed {
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove rewound file %s: %w", ref.Path, err)
			}
			continue
		}

		data, err := os.ReadFile(checkpointBackupPath(path, ref.BackupID))
		if err != nil {
			return fmt.Errorf("read checkpoint backup %s: %w", ref.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create rewind target dir %s: %w", ref.Path, err)
		}
		mode := os.FileMode(ref.Mode)
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(targetPath, data, mode); err != nil {
			return fmt.Errorf("restore rewound file %s: %w", ref.Path, err)
		}
	}
	return nil
}

func linkOrCopyFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func (s *Session) copyForkBackupsFrom(source *Session) error {
	if s == nil || source == nil {
		return nil
	}

	seen := make(map[string]struct{})
	for _, messageID := range s.checkpointOrder {
		record, ok := s.checkpoints[messageID]
		if !ok {
			continue
		}
		for _, ref := range record.Files {
			if strings.TrimSpace(ref.BackupID) == "" {
				continue
			}
			if _, ok := seen[ref.BackupID]; ok {
				continue
			}
			seen[ref.BackupID] = struct{}{}

			src := checkpointBackupPath(source.path, ref.BackupID)
			dst := checkpointBackupPath(s.path, ref.BackupID)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("create fork checkpoint dir: %w", err)
			}
			if err := linkOrCopyFile(src, dst); err != nil {
				return fmt.Errorf("copy checkpoint backup %s: %w", ref.BackupID, err)
			}
		}
	}
	return nil
}

// ForkFromCheckpoint creates a new session containing the visible history before the selected user message.
func (s *Session) ForkFromCheckpoint(messageID string) (*Session, error) {
	if s == nil {
		return nil, fmt.Errorf("session is nil")
	}

	s.mu.RLock()
	record, ok := s.checkpointRecordLocked(messageID)
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("checkpoint %s not found", strings.TrimSpace(messageID))
	}
	state, err := s.resumeStateForCheckpointLocked(record)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}

	workDir := s.meta.WorkDir
	if strings.TrimSpace(workDir) == "" {
		workDir = s.snapshot.WorkDir
	}
	key := s.meta.WorkDirKey
	if strings.TrimSpace(key) == "" {
		key = workDirKey(workDir)
	}
	now := time.Now()
	id, path, err := nextSessionLocation(key, now)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}

	fork := &Session{
		meta: Meta{
			Type:         recordTypeMeta,
			Version:      formatVersion,
			SessionID:    id,
			WorkDir:      workDir,
			WorkDirKey:   key,
			SystemPrompt: state.SystemPrompt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		records:         make([]MessageRecord, 0, len(s.records)),
		log:             make([]trajectoryEntry, 0, len(s.log)),
		resumeStates:    make([]ResumeStateRecord, 0, len(s.resumeStates)),
		checkpoints:     make(map[string]CheckpointRecord),
		checkpointOrder: make([]string, 0, len(s.checkpointOrder)),
		snapshot: Snapshot{
			SessionID:     id,
			WorkDir:       workDir,
			SystemPrompt:  state.SystemPrompt,
			UpdatedAt:     state.UpdatedAt,
			Messages:      cloneMessages(state.Messages),
			ProviderUsage: cloneUsageSnapshot(state.ProviderUsage),
		},
		path:         path,
		snapshotPath: snapshotPath(path),
	}

	for _, entry := range s.log {
		switch entry.kind {
		case recordTypeCheckpoint:
			if strings.TrimSpace(entry.checkpoint.MessageID) == strings.TrimSpace(messageID) {
				goto done
			}
		case recordTypeUser:
			if strings.TrimSpace(entry.message.MessageID) == strings.TrimSpace(messageID) {
				goto done
			}
		}
		fork.applyTrajectoryEntryLocked(entry)
	}

done:
	fork.activeCheckpointMessageID = ""
	fork.meta.SystemPrompt = state.SystemPrompt
	fork.snapshot = Snapshot{
		SessionID:     id,
		WorkDir:       workDir,
		SystemPrompt:  state.SystemPrompt,
		UpdatedAt:     state.UpdatedAt,
		Messages:      cloneMessages(state.Messages),
		ProviderUsage: cloneUsageSnapshot(state.ProviderUsage),
	}
	fork.bootstrapResumeState = false
	if err := fork.copyForkBackupsFrom(s); err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	s.mu.RUnlock()

	return fork, nil
}
