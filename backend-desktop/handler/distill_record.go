package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
)

func distillRecordsRoot() string {
	dir := filepath.Join(isolatedHome(), ".smart-agent", "distill-records")
	os.MkdirAll(dir, 0755)
	return dir
}

func distillRecordDir(id int64) string {
	return filepath.Join(distillRecordsRoot(), fmt.Sprintf("%d", id))
}

type persistDistillInput struct {
	RecordID        int64
	Family          string
	Alias           string
	Slug            string
	Profile         string
	PersonalityHint string
	Result          map[string]interface{}
	RawDir          string
}

func persistDistillRecord(in persistDistillInput) (int64, error) {
	sp, _ := in.Result["system_prompt"].(string)
	if sp == "" {
		sp = ""
	}
	name, _ := in.Result["name"].(string)
	desc, _ := in.Result["description"].(string)
	persJSON := ""
	if p, ok := in.Result["personality"]; ok {
		b, _ := json.Marshal(p)
		persJSON = string(b)
	}

	var recordID int64
	var version int
	var storageDir string
	existing, _ := db.GetDistillRecord(in.RecordID)
	if existing != nil && existing.ID > 0 {
		recordID = existing.ID
		version = existing.Version + 1
		storageDir = existing.StorageDir
		if storageDir == "" {
			storageDir = distillRecordDir(recordID)
		}
		_ = os.RemoveAll(filepath.Join(storageDir, "raw"))
		_ = os.RemoveAll(filepath.Join(storageDir, "output"))
	} else {
		placeholder := &db.DistillRecord{
			Family: in.Family, Alias: in.Alias, Slug: in.Slug,
			Profile: in.Profile, PersonalityHint: in.PersonalityHint,
			Version: 1, Status: "completed", StorageDir: "",
		}
		id, err := db.InsertDistillRecord(placeholder)
		if err != nil {
			return 0, err
		}
		recordID = id
		version = 1
		storageDir = distillRecordDir(recordID)
	}

	os.MkdirAll(filepath.Join(storageDir, "raw"), 0755)
	os.MkdirAll(filepath.Join(storageDir, "output"), 0755)

	sourceFiles, _ := copyDistillRawFiles(in.RawDir, filepath.Join(storageDir, "raw"))
	outputFiles, _ := copyDistillOutputFiles(in.Family, in.Slug, filepath.Join(storageDir, "output"))

	manifest := map[string]interface{}{
		"family": in.Family, "alias": in.Alias, "slug": in.Slug,
		"version": version, "record_id": recordID,
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(storageDir, "manifest.json"), mb, 0644)

	srcJSON, _ := json.Marshal(sourceFiles)
	outJSON, _ := json.Marshal(outputFiles)

	rec := &db.DistillRecord{
		ID: recordID, Family: in.Family, Alias: in.Alias, Slug: in.Slug,
		Profile: in.Profile, PersonalityHint: in.PersonalityHint,
		Name: name, Description: desc, SystemPrompt: sp,
		PersonalityJSON: persJSON,
		SourceFilesJSON: string(srcJSON),
		OutputFilesJSON: string(outJSON),
		StorageDir: storageDir, Version: version, Status: "completed",
	}
	rec.ID = recordID
	rec.StorageDir = storageDir
	if err := db.UpdateDistillRecord(rec); err != nil {
		return 0, err
	}
	return recordID, nil
}

func copyDistillRawFiles(rawDir, destRaw string) ([]db.DistillFileMeta, error) {
	var list []db.DistillFileMeta
	if rawDir == "" {
		return list, nil
	}
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return list, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "manifest.json" {
			continue
		}
		src := filepath.Join(rawDir, name)
		dst := filepath.Join(destRaw, name)
		if err := copyDistillFile(src, dst); err != nil {
			continue
		}
		info, _ := os.Stat(dst)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		list = append(list, db.DistillFileMeta{
			Name: name,
			Path: "raw/" + name,
			Size: size,
		})
	}
	return list, nil
}

func copyDistillOutputFiles(family, slug, destOut string) ([]db.DistillFileMeta, error) {
	var list []db.DistillFileMeta
	srcDir := distillSkillDir(family, slug)
	for _, name := range []string{"SKILL.md", "persona.md", "work.md"} {
		src := filepath.Join(srcDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(destOut, name)
		if err := copyDistillFile(src, dst); err != nil {
			continue
		}
		info, _ := os.Stat(dst)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		list = append(list, db.DistillFileMeta{
			Name: name,
			Path: "output/" + name,
			Size: size,
		})
	}
	return list, nil
}

func copyDistillFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func distillRecordToApplyMap(rec *db.DistillRecord) map[string]interface{} {
	out := map[string]interface{}{
		"record_id":     rec.ID,
		"family":        rec.Family,
		"slug":          rec.Slug,
		"name":          rec.Name,
		"description":   rec.Description,
		"system_prompt": rec.SystemPrompt,
		"avatar":        "✦",
	}
	if rec.PersonalityJSON != "" {
		var p map[string]interface{}
		if json.Unmarshal([]byte(rec.PersonalityJSON), &p) == nil {
			out["personality"] = p
		}
	}
	if rec.Alias != "" && out["name"] == "" {
		out["name"] = rec.Alias
	}
	return out
}

// ListDistillRecords GET /api/agents/distill/records
func ListDistillRecords(c *gin.Context) {
	list, err := db.ListDistillRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []db.DistillRecord{}
	}
	c.JSON(http.StatusOK, list)
}

// GetDistillRecordHandler GET /api/agents/distill/records/:id
func GetDistillRecordHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rec, err := db.GetDistillRecord(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rec == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	c.JSON(http.StatusOK, rec)
}

// DeleteDistillRecordHandler DELETE /api/agents/distill/records/:id
func DeleteDistillRecordHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rec, _ := db.GetDistillRecord(id)
	if rec != nil && rec.StorageDir != "" {
		os.RemoveAll(rec.StorageDir)
	}
	if err := db.DeleteDistillRecord(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ApplyDistillRecordHandler POST /api/agents/distill/records/:id/apply
func ApplyDistillRecordHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rec, err := db.GetDistillRecord(id)
	if err != nil || rec == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	c.JSON(http.StatusOK, distillRecordToApplyMap(rec))
}

// DownloadDistillRecordFile GET /api/agents/distill/records/:id/files/*filepath
func DownloadDistillRecordFile(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rel := c.Param("filepath")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效路径"})
		return
	}
	rec, err := db.GetDistillRecord(id)
	if err != nil || rec == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	full := filepath.Join(rec.StorageDir, rel)
	if _, err := os.Stat(full); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.File(full)
}
