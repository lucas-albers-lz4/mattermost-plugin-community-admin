package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

const MaxBatchRows = 200

type BatchRow struct {
	Username  string
	FirstName string
	LastName  string
	TeamName  string
	Channels  []string
}

type BatchRowResult struct {
	Username   string `json:"username"`
	Created    bool   `json:"created"`
	Skipped    bool   `json:"skipped"`
	Error      string `json:"error,omitempty"`
	Password   string `json:"password,omitempty"`
	ParentText string `json:"parent_text,omitempty"`
}

// CreateQuota checks whether another create is allowed (rate limit).
type CreateQuota func() (allowed bool, err error)

// ParseBatchCSV parses community-users CSV format.
func ParseBatchCSV(r io.Reader) ([]BatchRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have header and at least one row")
	}

	header := records[0]
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"username", "firstname", "lastname", "team"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("missing required column: %s", required)
		}
	}

	var rows []BatchRow
	for _, rec := range records[1:] {
		if len(rec) == 0 {
			continue
		}
		row := BatchRow{
			Username:  strings.TrimSpace(rec[col["username"]]),
			FirstName: strings.TrimSpace(rec[col["firstname"]]),
			LastName:  strings.TrimSpace(rec[col["lastname"]]),
			TeamName:  strings.TrimSpace(rec[col["team"]]),
		}
		if idx, ok := col["channels"]; ok && idx < len(rec) && strings.TrimSpace(rec[idx]) != "" {
			for ch := range strings.SplitSeq(rec[idx], ";") {
				ch = strings.TrimSpace(ch)
				if ch != "" {
					row.Channels = append(row.Channels, ch)
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// BatchImportService imports users scoped to organizer teams.
type BatchImportService struct {
	users   *UserService
	members *MembershipService
}

func NewBatchImportService(users *UserService, members *MembershipService) *BatchImportService {
	return &BatchImportService{users: users, members: members}
}

func buildTeamNameMap(org *config.Organizer) (map[string]string, error) {
	teamNameToID := map[string]string{}
	for _, t := range org.Teams {
		if prev, ok := teamNameToID[t.Name]; ok && prev != t.ID {
			return nil, fmt.Errorf("duplicate team name %q in organizer scope; use unique names or import via API with IDs", t.Name)
		}
		teamNameToID[t.Name] = t.ID
	}
	return teamNameToID, nil
}

func (s *BatchImportService) Import(rows []BatchRow, org *config.Organizer, cfg *config.ScopeConfig, dryRun bool, quota CreateQuota) ([]BatchRowResult, error) {
	if len(rows) > MaxBatchRows {
		return nil, fmt.Errorf("batch exceeds maximum of %d rows", MaxBatchRows)
	}

	teamNameToID, err := buildTeamNameMap(org)
	if err != nil {
		return nil, err
	}

	var results []BatchRowResult
	for _, row := range rows {
		res := BatchRowResult{Username: row.Username}
		if err := ValidateUsername(row.Username); err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		if row.FirstName == "" || row.LastName == "" {
			res.Error = "firstname and lastname required"
			results = append(results, res)
			continue
		}
		if row.TeamName == "" {
			res.Error = "team is required"
			results = append(results, res)
			continue
		}

		teamID, ok := teamNameToID[row.TeamName]
		if !ok {
			res.Error = fmt.Sprintf("team %q not in organizer scope", row.TeamName)
			results = append(results, res)
			continue
		}

		if dryRun {
			res.Skipped = true
			results = append(results, res)
			continue
		}

		existing, err := s.users.GetByUsername(row.Username)
		if err == nil && existing != nil {
			// Do not mutate membership for existing users — that bypasses authz/protected checks.
			res.Skipped = true
			res.Error = "user already exists (membership not modified; use membership API)"
			results = append(results, res)
			continue
		}

		channelIDs := []string{}
		for _, spec := range row.Channels {
			parts := strings.SplitN(spec, ":", 2)
			if len(parts) != 2 {
				res.Error = fmt.Sprintf("invalid channel spec %q", spec)
				break
			}
			found := false
			for _, ch := range org.Channels {
				if ch.Name == parts[1] {
					for _, t := range org.Teams {
						if t.Name == parts[0] && ch.TeamID == t.ID {
							channelIDs = append(channelIDs, ch.ID)
							found = true
							break
						}
					}
				}
			}
			if !found {
				for _, tid := range org.AllChannelsInTeams {
					for _, t := range org.Teams {
						if t.Name == parts[0] && t.ID == tid {
							res.Error = fmt.Sprintf("channel %q requires explicit scope entry for batch import", spec)
							break
						}
					}
				}
				if res.Error == "" {
					res.Error = fmt.Sprintf("channel %q not in organizer scope", spec)
				}
			}
		}
		if res.Error != "" {
			results = append(results, res)
			continue
		}

		if quota != nil {
			allowed, qerr := quota()
			if qerr != nil {
				return results, qerr
			}
			if !allowed {
				res.Error = "rate limit exceeded"
				results = append(results, res)
				continue
			}
		}

		created, err := s.users.CreateUser(CreateUserRequest{
			Username:   row.Username,
			FirstName:  row.FirstName,
			LastName:   row.LastName,
			TeamIDs:    []string{teamID},
			ChannelIDs: channelIDs,
		}, cfg.EmailDomain, cfg.SiteURL)
		if err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		res.Created = true
		res.Password = created.Password
		res.ParentText = created.ParentText
		results = append(results, res)
	}
	return results, nil
}
