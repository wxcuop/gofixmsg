# Archived GitHub Actions Workflows

This directory contains the original GitHub Actions workflows from the project before migration to GoFixMsg as the primary implementation.

## Archived Workflows

| Workflow | Purpose | Status |
|----------|---------|--------|
| `build_wheel.yml` | Build Python wheel distribution | Archived (Python pyfixmsg) |
| `ci.yml` | CI pipeline for Python code | Archived (Python pyfixmsg) |
| `deploy-docs.yml` | Deploy Sphinx documentation | Archived (Python docs) |
| `github_workflows_python-app-and-fix-demo.yml` | Python demo application | Archived (Python example) |
| `pyfixmsg-app.yml` | PyFixMsg application workflow | Archived (Python pyfixmsg) |
| `pyfixmsg_example.yaml` | PyFixMsg usage example | Archived (Python example) |
| `test_with_quickfix.yaml` | Python tests with QuickFIX spec | Archived (Python tests) |
| `test_with_quickfix_aiosqlite.yaml` | Python async tests with aiosqlite | Archived (Python tests) |

## New Workflows

The project now uses GoFixMsg with focused workflows:

| Workflow | Purpose | Location |
|----------|---------|----------|
| `publish-godoc.yml` | Publish GoDoc to gh_pages | `.github/workflows/` |
| `go-tests.yml` | Run Go tests on multiple OS/versions | `.github/workflows/` |

See `.github/workflows/` for current workflows.
