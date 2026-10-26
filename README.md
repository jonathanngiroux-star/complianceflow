# Compliance Automation

KYC/AML documentation cycles, MiCA/AMF regulatory-change tracking, weekly digests.

## Service Description

ComplianceFlow automates regulatory compliance for crypto businesses:

- **Multi-jurisdiction tracking** — EU (MiCA), France (AMF/ACPR), US (FinCEN), Singapore (MAS), UK (FCA)
- **Regulation monitoring** — RSS/API fetching of new/updated regulations
- **Document lifecycle** — KYC/AML policies, risk assessments, travel rule compliance docs
- **Review scheduling** — automated reminders for annual document reviews
- **Weekly digests** — FR/EN bilingual summaries with citations
- **Action tracking** — flags regulations requiring operational changes

Pricing: **$120–400/mo** (Specialty tier with SLA)

## Installation

```bash
go build -o complianceflow .
# or
go install github.com/jonathanngiroux-star/complianceflow@latest
```

## Configuration

```yaml
jurisdictions:
  - id: "eu"
    name: "European Union"
    code: "EU"
    regulator: "ESMA/EBA"
    enabled: true
    keywords: ["MiCA", "AML", "KYC", "travel rule"]

regulations:
  - id: "mica-2023"
    jurisdiction_id: "eu"
    title: "MiCA Regulation (EU) 2023/1114"
    category: "stablecoin"
    severity: "critical"
    action_required: true

documents:
  - id: "kyc-policy-1"
    entity_name: "MyCompany"
    jurisdiction: "eu"
    doc_type: "kyc_policy"
    status: "approved"
    version: "1.0"
    review_date: "2025-01-15"
```

## CLI Interface

```bash
# Add jurisdiction
complianceflow -action add-jurisdiction -name "Germany" -code "DE" -regulator "BaFin"

# Add regulation
complianceflow -action add-regulation -title "New AML Directive" -jurisdiction eu -category aml -severity high

# Add document
complianceflow -action add-document -entity "MyCompany" -doctype kyc_policy -jurisdiction eu

# List jurisdictions/regulations/documents
complianceflow -action list-jurisdictions
complianceflow -action list-regulations
complianceflow -action list-documents

# Generate weekly digest
complianceflow -action digest

# Check document reviews due
complianceflow -action reviews

# Run as daemon (fetches regulations + generates Friday digests)
complianceflow -action serve
```

## Supported Jurisdictions (Built-in)

| Code | Name | Regulator | Key Regulations |
|------|------|-----------|-----------------|
| EU | European Union | ESMA/EBA | MiCA, Travel Rule, AML Directives |
| FR | France | AMF/ACPR | PSAN, DASP, MiCA national law |
| US | United States | FinCEN/SEC/CFTC | BSA, Travel Rule, Stablecoin bills |
| SG | Singapore | MAS | Payment Services Act, Stablecoin framework |
| UK | United Kingdom | FCA | MLRs, Cryptoasset registration |

## Document Types

- `kyc_policy` — Customer Due Diligence procedures
- `aml_policy` — Anti-Money Laundering program
- `risk_assessment` — Enterprise-wide risk assessment
- `travel_rule_compliance` — Originator/beneficiary data collection
- `custody_policy` — Asset segregation and security

## Pricing

| Tier | Price | Includes |
|------|-------|----------|
| Starter | $120/mo | 3 jurisdictions, weekly digest, 10 documents |
| Professional | $250/mo | All jurisdictions, FR/EN digests, 50 documents, webhook alerts |
| Enterprise | $400/mo | Custom jurisdictions, SLA, dedicated compliance officer, on-call |

## Hosted Version

Managed compliance at **katsironni.com/complianceflow** — includes SLA, regulatory updates, bilingual (FR/EN) digests.

## License

MIT — see [LICENSE](LICENSE)