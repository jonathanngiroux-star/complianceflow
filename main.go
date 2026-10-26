package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Jurisdiction struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Code        string            `json:"code" yaml:"code"` // e.g., "EU", "FR", "US", "SG"
	Regulator   string            `json:"regulator" yaml:"regulator"`
	URL         string            `json:"url,omitempty" yaml:"url,omitempty"`
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	Keywords    []string          `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type Regulation struct {
	ID              string    `json:"id" yaml:"id"`
	JurisdictionID  string    `json:"jurisdiction_id" yaml:"jurisdiction_id"`
	Title           string    `json:"title" yaml:"title"`
	Description     string    `json:"description" yaml:"description"`
	SourceURL       string    `json:"source_url" yaml:"source_url"`
	PublishedAt     time.Time `json:"published_at" yaml:"published_at"`
	EffectiveAt     time.Time `json:"effective_at,omitempty" yaml:"effective_at,omitempty"`
	Category        string    `json:"category" yaml:"category"` // kyc, aml, travel_rule, stablecoin, defi, tax
	Severity        string    `json:"severity" yaml:"severity"` // critical, high, medium, low
	ActionRequired  bool      `json:"action_required" yaml:"action_required"`
	Summary         string    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Tags            []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Document struct {
	ID             string            `json:"id" yaml:"id"`
	EntityName     string            `json:"entity_name" yaml:"entity_name"`
	Jurisdiction   string            `json:"jurisdiction" yaml:"jurisdiction"`
	DocType        string            `json:"doc_type" yaml:"doc_type"` // kyc_policy, aml_policy, risk_assessment, travel_rule_compliance
	Status         string            `json:"status" yaml:"status"` // draft, review, approved, expired
	Version        string            `json:"version" yaml:"version"`
	Content        string            `json:"content,omitempty" yaml:"content,omitempty"`
	ContentHash    string            `json:"content_hash,omitempty" yaml:"content_hash,omitempty"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" yaml:"updated_at"`
	ReviewDate     time.Time         `json:"review_date,omitempty" yaml:"review_date,omitempty"`
	ApprovedBy     string            `json:"approved_by,omitempty" yaml:"approved_by,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type Config struct {
	Jurisdictions []Jurisdiction `json:"jurisdictions" yaml:"jurisdictions"`
	Regulations   []Regulation   `json:"regulations" yaml:"regulations"`
	Documents     []Document     `json:"documents" yaml:"documents"`
	DataDir       string         `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	FetchInterval string         `json:"fetch_interval,omitempty" yaml:"fetch_interval,omitempty"`
	DigestDay     int            `json:"digest_day,omitempty" yaml:"digest_day,omitempty"` // 0=Sunday, 1=Monday, etc.
	DigestTime    string         `json:"digest_time,omitempty" yaml:"digest_time,omitempty"` // HH:MM
	OutputDir     string         `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`
	WebhookURL    string         `json:"webhook_url,omitempty" yaml:"webhook_url,omitempty"`
}

type ComplianceFlow struct {
	config      *Config
	jurisdictions map[string]*Jurisdiction
	regulations   map[string]*Regulation
	documents     map[string]*Document
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	fetchTicker   *time.Ticker
	digestTicker  *time.Ticker
	wg            sync.WaitGroup
}

func NewComplianceFlow(configPath string) (*ComplianceFlow, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cf := &ComplianceFlow{
		config:        &Config{},
		jurisdictions: make(map[string]*Jurisdiction),
		regulations:   make(map[string]*Regulation),
		documents:     make(map[string]*Document),
		ctx:           ctx,
		cancel:        cancel,
	}

	if err := cf.LoadConfig(configPath); err != nil {
		return nil, err
	}

	for _, j := range cf.config.Jurisdictions {
		cf.jurisdictions[j.ID] = &j
	}
	for _, r := range cf.config.Regulations {
		cf.regulations[r.ID] = &r
	}
	for _, d := range cf.config.Documents {
		cf.documents[d.ID] = &d
	}

	if cf.config.DataDir == "" {
		cf.config.DataDir = "./data"
	}
	if cf.config.FetchInterval == "" {
		cf.config.FetchInterval = "24h"
	}
	if cf.config.DigestDay == 0 {
		cf.config.DigestDay = 5 // Friday
	}
	if cf.config.DigestTime == "" {
		cf.config.DigestTime = "09:00"
	}
	if cf.config.OutputDir == "" {
		cf.config.OutputDir = "./digests"
	}

	os.MkdirAll(cf.config.DataDir, 0755)
	os.MkdirAll(cf.config.OutputDir, 0755)

	return cf, nil
}

func (cf *ComplianceFlow) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cf.CreateDefaultConfig(path)
		}
		return err
	}

	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return yaml.Unmarshal(data, cf.config)
	}
	return json.Unmarshal(data, cf.config)
}

func (cf *ComplianceFlow) CreateDefaultConfig(path string) error {
	cf.config = &Config{
		Jurisdictions: []Jurisdiction{
			{ID: "eu", Name: "European Union", Code: "EU", Regulator: "ESMA/EBA", URL: "https://www.esma.europa.eu", Enabled: true, Keywords: []string{"MiCA", "AML", "KYC", "travel rule"}},
			{ID: "fr", Name: "France", Code: "FR", Regulator: "AMF/ACPR", URL: "https://www.amf-france.org", Enabled: true, Keywords: []string{"MiCA", "PSAN", "DASP"}},
			{ID: "us", Name: "United States", Code: "US", Regulator: "FinCEN/SEC/CFTC", URL: "https://www.fincen.gov", Enabled: true, Keywords: []string{"BSA", "KYC", "AML", "travel rule"}},
			{ID: "sg", Name: "Singapore", Code: "SG", Regulator: "MAS", URL: "https://www.mas.gov.sg", Enabled: true, Keywords: []string{"PS Act", "stablecoin", "DPT"}},
			{ID: "uk", Name: "United Kingdom", Code: "UK", Regulator: "FCA", URL: "https://www.fca.org.uk", Enabled: true, Keywords: []string{"MLR", "cryptoasset", "travel rule"}},
		},
		Regulations: []Regulation{
			{
				ID:             "mica-2023",
				JurisdictionID: "eu",
				Title:          "MiCA Regulation (EU) 2023/1114",
				Description:    "Markets in Crypto-Assets Regulation - comprehensive framework for crypto-assets",
				SourceURL:      "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32023R1114",
				PublishedAt:    time.Date(2023, 6, 9, 0, 0, 0, 0, time.UTC),
				EffectiveAt:    time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC),
				Category:       "stablecoin",
				Severity:       "critical",
				ActionRequired: true,
				Tags:           []string{"casps", "stablecoins", "whitepaper", "governance"},
			},
			{
				ID:             "travel-rule-eu",
				JurisdictionID: "eu",
				Title:          "EU Travel Rule (Regulation 2023/1113)",
				Description:    "Information accompanying transfers of funds and crypto-assets",
				SourceURL:      "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32023R1113",
				PublishedAt:    time.Date(2023, 6, 9, 0, 0, 0, 0, time.UTC),
				EffectiveAt:    time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC),
				Category:       "travel_rule",
				Severity:       "critical",
				ActionRequired: true,
				Tags:           []string{"originator", "beneficiary", "VASP"},
			},
			{
				ID:             "fr-psan",
				JurisdictionID: "fr",
				Title:          "PSAN Registration (AMF)",
				Description:    "Prestataire de Services sur Actifs Numériques - mandatory registration for crypto service providers",
				SourceURL:      "https://www.amf-france.org/fr/reglementation/actifs-numeriques/psan",
				PublishedAt:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
				Category:       "kyc",
				Severity:       "high",
				ActionRequired: true,
				Tags:           []string{"registration", "custody", "exchange", "broker"},
			},
		},
		Documents: []Document{
			{
				ID:          "kyc-template-eu",
				EntityName:  "Template",
				Jurisdiction: "eu",
				DocType:     "kyc_policy",
				Status:      "approved",
				Version:     "1.0",
				Content:     "# EU KYC Policy Template\n\n## Customer Due Diligence\n...\n",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				ReviewDate:  time.Now().AddDate(1, 0, 0),
			},
		},
		FetchInterval: "24h",
		DigestDay:     5,
		DigestTime:    "09:00",
		OutputDir:     "./digests",
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := yaml.Marshal(cf.config)
	return os.WriteFile(path, data, 0644)
}

func (cf *ComplianceFlow) SaveConfig() error {
	cf.mu.RLock()
	cf.config.Jurisdictions = make([]Jurisdiction, 0, len(cf.jurisdictions))
	for _, j := range cf.jurisdictions {
		cf.config.Jurisdictions = append(cf.config.Jurisdictions, *j)
	}
	cf.config.Regulations = make([]Regulation, 0, len(cf.regulations))
	for _, r := range cf.regulations {
		cf.config.Regulations = append(cf.config.Regulations, *r)
	}
	cf.config.Documents = make([]Document, 0, len(cf.documents))
	for _, d := range cf.documents {
		cf.config.Documents = append(cf.config.Documents, *d)
	}
	data, _ := yaml.Marshal(cf.config)
	cf.mu.RUnlock()
	return os.WriteFile(cf.configPath(), data, 0644)
}

func (cf *ComplianceFlow) configPath() string {
	return "./config.yaml"
}

func (cf *ComplianceFlow) AddJurisdiction(j *Jurisdiction) {
	if j.ID == "" {
		j.ID = strings.ToLower(strings.ReplaceAll(j.Name, " ", "-"))
	}
	cf.mu.Lock()
	cf.jurisdictions[j.ID] = j
	cf.config.Jurisdictions = append(cf.config.Jurisdictions, *j)
	cf.mu.Unlock()
	cf.SaveConfig()
}

func (cf *ComplianceFlow) AddRegulation(r *Regulation) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	cf.mu.Lock()
	cf.regulations[r.ID] = r
	cf.config.Regulations = append(cf.config.Regulations, *r)
	cf.mu.Unlock()
	cf.SaveConfig()
}

func (cf *ComplianceFlow) AddDocument(d *Document) {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	d.ContentHash = hashContent(d.Content)
	cf.mu.Lock()
	cf.documents[d.ID] = d
	cf.config.Documents = append(cf.config.Documents, *d)
	cf.mu.Unlock()
	cf.SaveConfig()
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

func (cf *ComplianceFlow) ListJurisdictions() []*Jurisdiction {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	list := make([]*Jurisdiction, 0, len(cf.jurisdictions))
	for _, j := range cf.jurisdictions {
		list = append(list, j)
	}
	return list
}

func (cf *ComplianceFlow) ListRegulations(jurisdictionID string) []*Regulation {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	list := make([]*Regulation, 0)
	for _, r := range cf.regulations {
		if jurisdictionID == "" || r.JurisdictionID == jurisdictionID {
			list = append(list, r)
		}
	}
	return list
}

func (cf *ComplianceFlow) ListDocuments(entityName string) []*Document {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	list := make([]*Document, 0)
	for _, d := range cf.documents {
		if entityName == "" || d.EntityName == entityName {
			list = append(list, d)
		}
	}
	return list
}

func (cf *ComplianceFlow) GenerateWeeklyDigest() (string, error) {
	cf.mu.RLock()
	regs := make([]*Regulation, 0, len(cf.regulations))
	for _, r := range cf.regulations {
		// Only include regulations from last 7 days
		if time.Since(r.PublishedAt) <= 7*24*time.Hour {
			regs = append(regs, r)
		}
	}
	cf.mu.RUnlock()

	if len(regs) == 0 {
		return "No new regulations this week.", nil
	}

	// Group by jurisdiction
	byJurisdiction := make(map[string][]*Regulation)
	for _, r := range regs {
		byJurisdiction[r.JurisdictionID] = append(byJurisdiction[r.JurisdictionID], r)
	}

	var digest strings.Builder
	digest.WriteString(fmt.Sprintf("# Weekly Compliance Digest - Week of %s\n\n", time.Now().Format("Jan 2, 2006")))
	digest.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	for jurID, regs := range byJurisdiction {
		jur := cf.jurisdictions[jurID]
		jurName := jurID
		if jur != nil {
			jurName = jur.Name
		}
		digest.WriteString(fmt.Sprintf("## %s (%d updates)\n\n", jurName, len(regs)))
		for _, r := range regs {
			digest.WriteString(fmt.Sprintf("### %s [%s]\n", r.Title, strings.ToUpper(r.Severity)))
			digest.WriteString(fmt.Sprintf("- **Category**: %s\n", r.Category))
			digest.WriteString(fmt.Sprintf("- **Published**: %s\n", r.PublishedAt.Format("Jan 2, 2006")))
			if !r.EffectiveAt.IsZero() {
				digest.WriteString(fmt.Sprintf("- **Effective**: %s\n", r.EffectiveAt.Format("Jan 2, 2006")))
			}
			digest.WriteString(fmt.Sprintf("- **Action Required**: %v\n", r.ActionRequired))
			digest.WriteString(fmt.Sprintf("- **Summary**: %s\n", r.Summary))
			digest.WriteString(fmt.Sprintf("- **Source**: %s\n\n", r.SourceURL))
		}
	}

	// Save digest
	filename := fmt.Sprintf("digest_%s.md", time.Now().Format("2006-01-02"))
	path := filepath.Join(cf.config.OutputDir, filename)
	os.MkdirAll(cf.config.OutputDir, 0755)
	os.WriteFile(path, []byte(digest.String()), 0644)

	return digest.String(), nil
}

func (cf *ComplianceFlow) CheckDocumentReviews() []*Document {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	var due []*Document
	for _, d := range cf.documents {
		if !d.ReviewDate.IsZero() && time.Now().After(d.ReviewDate) && d.Status != "expired" {
			due = append(due, d)
		}
	}
	return due
}

func (cf *ComplianceFlow) StartSchedulers() {
	fetchInterval, _ := time.ParseDuration(cf.config.FetchInterval)
	if fetchInterval == 0 {
		fetchInterval = 24 * time.Hour
	}

	cf.fetchTicker = time.NewTicker(fetchInterval)
	cf.wg.Add(1)
	go func() {
		defer cf.wg.Done()
		for {
			select {
			case <-cf.ctx.Done():
				return
			case <-cf.fetchTicker.C:
				cf.fetchRegulations()
			}
		}
	}()

	// Daily digest check
	cf.digestTicker = time.NewTicker(1 * time.Hour)
	cf.wg.Add(1)
	go func() {
		defer cf.wg.Done()
		for {
			select {
			case <-cf.ctx.Done():
				return
			case <-cf.digestTicker.C:
				now := time.Now()
				if int(now.Weekday()) == cf.config.DigestDay && now.Format("15:04") == cf.config.DigestTime {
					digest, _ := cf.GenerateWeeklyDigest()
					log.Printf("Generated weekly digest:\n%s", digest)
					if cf.config.WebhookURL != "" {
						cf.sendWebhook(digest)
					}
				}
			}
		}
	}()
}

func (cf *ComplianceFlow) fetchRegulations() {
	// In production, this would fetch from regulatory RSS feeds, APIs, etc.
	log.Println("Fetching regulatory updates...")
}

func (cf *ComplianceFlow) sendWebhook(content string) {
	// Send digest to webhook
}

func (cf *ComplianceFlow) Stop() {
	if cf.fetchTicker != nil {
		cf.fetchTicker.Stop()
	}
	if cf.digestTicker != nil {
		cf.digestTicker.Stop()
	}
	cf.cancel()
	cf.wg.Wait()
}

func main() {
	var (
		configPath = flag.String("config", "./config.yaml", "Path to config file")
		action     = flag.String("action", "", "Action: add-jurisdiction, add-regulation, add-document, list-jurisdictions, list-regulations, list-documents, digest, reviews, serve")
		name       = flag.String("name", "", "Name (for add)")
		code       = flag.String("code", "", "Jurisdiction code")
		regulator  = flag.String("regulator", "", "Regulator name")
		url        = flag.String("url", "", "Source URL")
		jurisdiction = flag.String("jurisdiction", "", "Jurisdiction ID")
		title      = flag.String("title", "", "Regulation title")
		category   = flag.String("category", "", "Category")
		severity   = flag.String("severity", "medium", "Severity")
		entityName = flag.String("entity", "", "Entity name")
		docType    = flag.String("doctype", "", "Document type")
	)
	flag.Parse()

	cf, err := NewComplianceFlow(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize ComplianceFlow: %v", err)
	}
	defer cf.Stop()

	switch *action {
	case "add-jurisdiction":
		if *name == "" || *code == "" {
			log.Fatal("name and code required")
		}
		j := &Jurisdiction{Name: *name, Code: *code, Regulator: *regulator, URL: *url, Enabled: true}
		cf.AddJurisdiction(j)
		fmt.Printf("Added jurisdiction: %s (%s)\n", j.Name, j.ID)

	case "add-regulation":
		if *title == "" || *jurisdiction == "" || *category == "" {
			log.Fatal("title, jurisdiction, and category required")
		}
		r := &Regulation{
			JurisdictionID: *jurisdiction,
			Title:          *title,
			Category:       *category,
			Severity:       *severity,
			SourceURL:      *url,
			PublishedAt:    time.Now(),
		}
		cf.AddRegulation(r)
		fmt.Printf("Added regulation: %s (%s)\n", r.Title, r.ID)

	case "add-document":
		if *entityName == "" || *docType == "" || *jurisdiction == "" {
			log.Fatal("entity, doctype, and jurisdiction required")
		}
		d := &Document{
			EntityName:  *entityName,
			Jurisdiction: *jurisdiction,
			DocType:     *docType,
			Status:      "draft",
			Version:     "1.0",
		}
		cf.AddDocument(d)
		fmt.Printf("Added document: %s (%s)\n", d.EntityName, d.ID)

	case "list-jurisdictions":
		for _, j := range cf.ListJurisdictions() {
			status := "enabled"
			if !j.Enabled {
				status = "disabled"
			}
			fmt.Printf("%s | %s | %s | %s\n", j.ID, j.Name, j.Code, status)
		}

	case "list-regulations":
		regs := cf.ListRegulations(*jurisdiction)
		for _, r := range regs {
			fmt.Printf("%s | %s | %s | %s | %s\n", r.ID[:8], r.Title, r.JurisdictionID, r.Category, r.Severity)
		}

	case "list-documents":
		docs := cf.ListDocuments(*entityName)
		for _, d := range docs {
			fmt.Printf("%s | %s | %s | %s | %s | review: %s\n",
				d.ID[:8], d.EntityName, d.Jurisdiction, d.DocType, d.Status, d.ReviewDate.Format("2006-01-02"))
		}

	case "digest":
		digest, _ := cf.GenerateWeeklyDigest()
		fmt.Println(digest)

	case "reviews":
		due := cf.CheckDocumentReviews()
		if len(due) == 0 {
			fmt.Println("No documents due for review")
			return
		}
		for _, d := range due {
			fmt.Printf("%s | %s | %s | due: %s\n", d.ID[:8], d.EntityName, d.DocType, d.ReviewDate.Format("2006-01-02"))
		}

	case "serve":
		cf.StartSchedulers()
		<-cf.ctx.Done()

	default:
		flag.Usage()
		os.Exit(1)
	}
}