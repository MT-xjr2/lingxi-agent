package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

type DistillRecord struct {
	ID              int64  `json:"id"`
	Family          string `json:"family"`
	Alias           string `json:"alias"`
	Slug            string `json:"slug"`
	Profile         string `json:"profile"`
	PersonalityHint string `json:"personality_hint"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	SystemPrompt    string `json:"system_prompt"`
	PersonalityJSON string `json:"personality_json"`
	SourceFilesJSON string `json:"source_files_json"`
	OutputFilesJSON string `json:"output_files_json"`
	StorageDir      string `json:"storage_dir"`
	Version         int    `json:"version"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type DistillFileMeta struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size,omitempty"`
}

func InsertDistillRecord(r *DistillRecord) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	res, err := DB.Exec(`
		INSERT INTO agent_distill_records (
			family, alias, slug, profile, personality_hint,
			name, description, system_prompt, personality_json,
			source_files_json, output_files_json, storage_dir,
			version, status, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.Family, r.Alias, r.Slug, r.Profile, r.PersonalityHint,
		r.Name, r.Description, r.SystemPrompt, r.PersonalityJSON,
		r.SourceFilesJSON, r.OutputFilesJSON, r.StorageDir,
		r.Version, r.Status, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateDistillRecord(r *DistillRecord) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := DB.Exec(`
		UPDATE agent_distill_records SET
			family=?, alias=?, slug=?, profile=?, personality_hint=?,
			name=?, description=?, system_prompt=?, personality_json=?,
			source_files_json=?, output_files_json=?, storage_dir=?,
			version=?, status=?, updated_at=?
		WHERE id=?`,
		r.Family, r.Alias, r.Slug, r.Profile, r.PersonalityHint,
		r.Name, r.Description, r.SystemPrompt, r.PersonalityJSON,
		r.SourceFilesJSON, r.OutputFilesJSON, r.StorageDir,
		r.Version, r.Status, now, r.ID,
	)
	return err
}

func ListDistillRecords() ([]DistillRecord, error) {
	rows, err := DB.Query(`
		SELECT id, family, alias, slug, profile, personality_hint,
			name, description,
			CASE WHEN length(system_prompt) > 200 THEN substr(system_prompt,1,200)||'…' ELSE system_prompt END,
			personality_json, source_files_json, output_files_json,
			storage_dir, version, status, created_at, updated_at
		FROM agent_distill_records
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []DistillRecord
	for rows.Next() {
		var r DistillRecord
		if err := rows.Scan(
			&r.ID, &r.Family, &r.Alias, &r.Slug, &r.Profile, &r.PersonalityHint,
			&r.Name, &r.Description, &r.SystemPrompt,
			&r.PersonalityJSON, &r.SourceFilesJSON, &r.OutputFilesJSON,
			&r.StorageDir, &r.Version, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			continue
		}
		list = append(list, r)
	}
	return list, nil
}

func GetDistillRecord(id int64) (*DistillRecord, error) {
	var r DistillRecord
	err := DB.QueryRow(`
		SELECT id, family, alias, slug, profile, personality_hint,
			name, description, system_prompt, personality_json,
			source_files_json, output_files_json, storage_dir,
			version, status, created_at, updated_at
		FROM agent_distill_records WHERE id=?`, id).Scan(
		&r.ID, &r.Family, &r.Alias, &r.Slug, &r.Profile, &r.PersonalityHint,
		&r.Name, &r.Description, &r.SystemPrompt, &r.PersonalityJSON,
		&r.SourceFilesJSON, &r.OutputFilesJSON, &r.StorageDir,
		&r.Version, &r.Status, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func DeleteDistillRecord(id int64) error {
	_, err := DB.Exec(`DELETE FROM agent_distill_records WHERE id=?`, id)
	return err
}

func ParseDistillFileList(jsonStr string) []DistillFileMeta {
	var list []DistillFileMeta
	if jsonStr == "" {
		return list
	}
	_ = json.Unmarshal([]byte(jsonStr), &list)
	return list
}
