# Acme Security, Compliance & Data Governance Policy

## Overview
This document specifies access controls, authentication standards, encryption requirements, and data classification guidelines enforced across Acme's systems.

## Identity and Access Management (IAM)

### Password Requirements
- Minimum length: **14 characters**.
- Complexity: Must contain at least one uppercase letter, one lowercase letter, one numeric digit, and one special symbol (`!@#$%^&*`).
- Passwords expire every **90 days** and cannot match any of the previous 8 passwords.
- Account lockout occurs after **5 consecutive failed attempts** for a duration of 30 minutes.

### Multi-Factor Authentication (MFA)
- MFA is **strictly mandatory** for all corporate accounts, VPNs, cloud consoles, and source code repositories.
- Approved MFA factors: FIDO2/WebAuthn hardware security keys (e.g., YubiKey) and TOTP authenticator applications (Google Authenticator, 1Password).
- SMS and voice call verification are prohibited due to SIM-swapping vulnerabilities.

## Data Classification Tiers

| Tier | Classification | Description | Examples | Handling Requirements |
| --- | --- | --- | --- | --- |
| **Tier 1** | **Public** | Information approved for public distribution | Marketing blogs, open source docs | No special restriction |
| **Tier 2** | **Internal** | General internal company communications | Internal wikis, Slack discussions | Accessible to all Acme employees |
| **Tier 3** | **Confidential** | Sensitive operational and customer business data | Customer contracts, source code, PII | Role-based access control, TLS + at-rest encryption |
| **Tier 4** | **Restricted** | High-impact secrets and regulated data | Encryption master keys, payment credentials | Just-In-Time access approval, audited logs |

## Cryptography & Encryption Standards
1. **Data in Transit:** All external and internal service-to-service communications must enforce **TLS 1.3** (or TLS 1.2 with forward secrecy cipher suites). Unencrypted HTTP is prohibited in production.
2. **Data at Rest:** All databases, file stores, and backups must use **AES-256-GCM** encryption. Key rotation occurs annually via KMS.
3. **Endpoint Protection:** All company laptops must enforce full-disk encryption (FileVault for macOS, BitLocker for Windows). Screen locks engage automatically after **5 minutes of inactivity**.

## Security Incident Reporting
- Any suspected security incident, unauthorized data disclosure, or lost company device must be reported within **1 hour** of discovery to `security@acme.example.com` or via the Slack channel `#incident-security`.
- The incident response team maintains a maximum 15-minute triage SLA for reported breaches.
