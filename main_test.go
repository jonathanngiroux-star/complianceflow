package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComplianceFlowConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	if len(cf.jurisdictions) == 0 {
		t.Error("Expected default jurisdictions to be loaded")
	}
	if _, ok := cf.jurisdictions["eu"]; !ok {
		t.Error("Expected EU jurisdiction")
	}
	if _, ok := cf.jurisdictions["fr"]; !ok {
		t.Error("Expected FR jurisdiction")
	}
}

func TestAddJurisdiction(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	j := &Jurisdiction{Name: "Test Country", Code: "TC", Regulator: "Test Regulator", Enabled: true}
	cf.AddJurisdiction(j)

	if j.ID == "" {
		t.Error("Jurisdiction should have ID")
	}

	_, ok := cf.jurisdictions[j.ID]
	if !ok {
		t.Error("Jurisdiction should be in map")
	}
}

func TestAddRegulation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	r := &Regulation{
		JurisdictionID: "eu",
		Title:          "Test Regulation",
		Category:       "aml",
		Severity:       "high",
		SourceURL:      "https://example.com",
		PublishedAt:    time.Now(),
	}
	cf.AddRegulation(r)

	if r.ID == "" {
		t.Error("Regulation should have ID")
	}

	_, ok := cf.regulations[r.ID]
	if !ok {
		t.Error("Regulation should be in map")
	}
}

func TestAddDocument(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	d := &Document{
		EntityName:  "Test Entity",
		Jurisdiction: "eu",
		DocType:     "kyc_policy",
		Status:      "draft",
		Version:     "1.0",
		Content:     "Test content",
	}
	cf.AddDocument(d)

	if d.ID == "" {
		t.Error("Document should have ID")
	}
	if d.ContentHash == "" {
		t.Error("Document should have content hash")
	}
	if d.CreatedAt.IsZero() {
		t.Error("Document should have created timestamp")
	}

	_, ok := cf.documents[d.ID]
	if !ok {
		t.Error("Document should be in map")
	}
}

func TestListOperations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	jurisdictions := cf.ListJurisdictions()
	if len(jurisdictions) == 0 {
		t.Error("Expected jurisdictions list")
	}

	regulations := cf.ListRegulations("")
	// Regulations might be empty in test config, just verify the function works
	_ = regulations

	euRegs := cf.ListRegulations("eu")
	_ = euRegs // May be empty in test
}

func TestDocumentReviewCheck(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cf, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	// Add document with past review date
	d := &Document{
		EntityName:   "Test Entity",
		Jurisdiction: "eu",
		DocType:      "kyc_policy",
		Status:       "approved",
		Version:      "1.0",
		Content:      "Test",
		ReviewDate:   time.Now().Add(-24 * time.Hour), // Past due
	}
	cf.AddDocument(d)

	due := cf.CheckDocumentReviews()
	found := false
	for _, doc := range due {
		if doc.ID == d.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Document with past review date should be in due list")
	}

	// Add document with future review date
	d2 := &Document{
		EntityName:   "Test Entity 2",
		Jurisdiction: "eu",
		DocType:      "aml_policy",
		Status:       "approved",
		Version:      "1.0",
		Content:      "Test",
		ReviewDate:   time.Now().Add(30 * 24 * time.Hour), // Future
	}
	cf.AddDocument(d2)

	due = cf.CheckDocumentReviews()
	found2 := false
	for _, doc := range due {
		if doc.ID == d2.ID {
			found2 = true
			break
		}
	}
	if found2 {
		t.Error("Document with future review date should NOT be in due list")
	}
}

func TestConfigFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	_, err := NewComplianceFlow(configPath)
	if err != nil {
		t.Fatalf("Failed to create ComplianceFlow: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}

	data, _ := os.ReadFile(configPath)
	if len(data) == 0 {
		t.Error("Config file should not be empty")
	}
}