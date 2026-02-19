package amadeus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DMailStatus represents the lifecycle status of a D-Mail.
type DMailStatus string

const (
	// DMailPending indicates a D-Mail awaiting human approval.
	DMailPending DMailStatus = "pending"
	// DMailSent indicates a D-Mail that has been auto-sent.
	DMailSent DMailStatus = "sent"
	// DMailApproved indicates a D-Mail approved by a human.
	DMailApproved DMailStatus = "approved"
	// DMailRejected indicates a D-Mail rejected by a human.
	DMailRejected DMailStatus = "rejected"
)

// DMailTarget represents the destination tool for a D-Mail.
type DMailTarget string

const (
	// TargetSightjack routes the D-Mail to the Sightjack tool.
	TargetSightjack DMailTarget = "sightjack"
	// TargetPaintress routes the D-Mail to the Paintress tool.
	TargetPaintress DMailTarget = "paintress"
)

// DMail is the correction routing message produced by Phase 3.
type DMail struct {
	ID             string      `json:"id"`
	Severity       Severity    `json:"severity"`
	Status         DMailStatus `json:"status"`
	Target         DMailTarget `json:"target"`
	Type           string      `json:"type"`
	Summary        string      `json:"summary"`
	Detail         string      `json:"detail"`
	CreatedAt      time.Time   `json:"created_at"`
	ResolvedAt     *time.Time  `json:"resolved_at,omitempty"`
	ResolvedAction *string     `json:"resolved_action,omitempty"`
	RejectReason   *string     `json:"reject_reason,omitempty"`
}

// RouteDMail applies severity-based status mapping.
// HIGH severity requires human approval (pending); all others are auto-sent.
func RouteDMail(dmail DMail) DMail {
	switch dmail.Severity {
	case SeverityHigh:
		dmail.Status = DMailPending
	default:
		dmail.Status = DMailSent
	}
	return dmail
}

// NextDMailID returns the next sequential D-Mail ID by scanning existing files
// in the dmails/ directory.
func (s *StateStore) NextDMailID() (string, error) {
	dmailDir := filepath.Join(s.Root, "dmails")
	entries, err := os.ReadDir(dmailDir)
	if err != nil {
		return "", err
	}
	maxNum := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.HasPrefix(name, "d-") {
			var num int
			if _, err := fmt.Sscanf(name, "d-%d", &num); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("d-%03d", maxNum+1), nil
}

// LoadDMail reads a single D-Mail by ID from the dmails/ directory.
func (s *StateStore) LoadDMail(id string) (DMail, error) {
	path := filepath.Join(s.Root, "dmails", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return DMail{}, fmt.Errorf("load dmail %s: %w", id, err)
	}
	var dmail DMail
	if err := json.Unmarshal(data, &dmail); err != nil {
		return DMail{}, fmt.Errorf("parse dmail %s: %w", id, err)
	}
	return dmail, nil
}

// SaveDMail writes a D-Mail to the dmails/ directory as JSON.
func (s *StateStore) SaveDMail(dmail DMail) error {
	path := filepath.Join(s.Root, "dmails", dmail.ID+".json")
	return s.writeJSON(path, dmail)
}

// LoadAllDMails reads all D-Mails from the dmails/ directory, sorted by ID ascending.
func (s *StateStore) LoadAllDMails() ([]DMail, error) {
	dmailDir := filepath.Join(s.Root, "dmails")
	entries, err := os.ReadDir(dmailDir)
	if err != nil {
		return nil, err
	}
	var dmails []DMail
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		dmail, err := s.LoadDMail(id)
		if err != nil {
			return nil, err
		}
		dmails = append(dmails, dmail)
	}
	sort.Slice(dmails, func(i, j int) bool {
		return dmails[i].ID < dmails[j].ID
	})
	return dmails, nil
}
