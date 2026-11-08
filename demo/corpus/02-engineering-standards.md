# Acme Engineering Standards & Operational Guidelines

## Purpose
This document outlines technical conventions, development workflows, quality gates, and operational procedures for all Acme Engineering teams.

## Source Control and Git Workflows

### Branching Strategy
We follow Trunk-Based Development with short-lived feature branches:
- **Main Branch:** The `main` branch represents deployable production code. Direct pushes to `main` are disabled via branch protection.
- **Branch Naming:** Feature branches must follow the convention `<type>/<ticket-id>-<short-description>`, e.g., `feat/KF-37-rag-eval` or `fix/AUTH-04-token-expiry`.
- **Pull Request Lifetime:** Pull requests should remain open for less than 48 hours and encompass no more than 400 lines of functional diff (excluding generated code and tests).

### Commit Message Conventions
Commits must adhere to Conventional Commits:
- `feat:` New user-facing or platform features.
- `fix:` Bug fixes.
- `test:` Adding or updating automated test suites.
- `docs:` Documentation, architecture guides, and user manuals.
- `refactor:` Code restructuring without behavior changes.
- `chore:` Dependency bumps, CI maintenance, and build tooling.

## Code Review SLAs and Reviewer Etiquette
- **Review SLA:** Pull requests must receive an initial review within **4 business hours** of assignment.
- **Approvals Required:** At least two peer approvals are mandatory for backend core services; at least one peer approval for documentation and scripts.
- **Blocking Feedback:** Reviewers must mark non-critical suggestions with `nit:` so authors know they can merge without blocking.

## Testing Standards & Quality Gates
1. **Unit Testing:** All business logic packages (services, parsing, helpers) must have table-driven unit tests with minimum **80% line coverage**.
2. **Integration Testing:** Repositories and external service wrappers must be verified against real dependencies (e.g. PostgreSQL with pgvector) rather than in-memory mocks.
3. **Flaky Tests:** Any test that fails intermittently without code changes must be quarantined within 24 hours.
4. **CI Pipeline:** Every commit pushed triggers linting (`golangci-lint`), static analysis (`go vet`), unit tests, and multi-stage Docker builds. No PR may merge with failing CI checks.

## Incident Management & Severity Levels

| Severity | Definition | Response SLA | Update Cadence | Post-Mortem |
| --- | --- | --- | --- | --- |
| **SEV-1 (Critical)** | Core service down for >25% of users; data loss risk | **15 minutes** | Every 30 minutes | Mandatory within 48h |
| **SEV-2 (Major)** | Major functionality degraded; no workaround | **30 minutes** | Every 1 hour | Mandatory within 72h |
| **SEV-3 (Moderate)** | Non-critical feature failure with workaround | **4 business hours** | Daily | Recommended |
| **SEV-4 (Minor)** | Minor bug, cosmetic UI glitch | **2 business days** | Weekly | Not required |

### On-Call Rotation
- Primary and secondary on-call engineers rotate weekly every Tuesday at 10:00 AM UTC.
- Pager alerts escalate to secondary if unacknowledged within 10 minutes for SEV-1 and SEV-2 incidents.
- Engineers on primary on-call receive a **$400 weekly shift stipend** plus compensatory time off for out-of-hours incident mitigations.
