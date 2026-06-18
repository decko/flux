#!/bin/bash
# Setup GitHub structure for flux project
# Run this after creating the repo: ./scripts/setup-github.sh

set -e

REPO="decko/flux"

echo "Setting up GitHub structure for $REPO..."

# Create milestones
echo "Creating milestones..."
gh api repos/$REPO/milestones -f title="M1: Foundation" -f description="Go + Chi + SQLite + basic API"
gh api repos/$REPO/milestones -f title="M2: GitHub Adapter" -f description="Issues + PRs sync from GitHub"
gh api repos/$REPO/milestones -f title="M3: soda Integration" -f description="Trigger and monitor soda runs"
gh api repos/$REPO/milestones -f title="M4: Frontend" -f description="SPA with TanStack Router/Query"
gh api repos/$REPO/milestones -f title="M5: Self-Host" -f description="Flux manages flux"

# Create labels
echo "Creating labels..."

# Area labels
gh api repos/$REPO/labels -f name="area/backend" -f color="0075ca" -f description="Go backend work"
gh api repos/$REPO/labels -f name="area/frontend" -f color="7057ff" -f description="TypeScript/React work"
gh api repos/$REPO/labels -f name="area/adapter" -f color="0e8a16" -f description="Adapter implementations"
gh api repos/$REPO/labels -f name="area/api" -f color="fbca04" -f description="API endpoints"
gh api repos/$REPO/labels -f name="area/database" -f color="d93f0b" -f description="Database/migrations"
gh api repos/$REPO/labels -f name="area/docs" -f color="006b75" -f description="Documentation"

# Type labels
gh api repos/$REPO/labels -f name="type/feature" -f color="a2eeef" -f description="New feature"
gh api repos/$REPO/labels -f name="type/bug" -f color="d73a4a" -f description="Bug fix"
gh api repos/$REPO/labels -f name="type/refactor" -f color="c5def5" -f description="Code refactoring"
gh api repos/$REPO/labels -f name="type/test" -f color="e4e669" -f description="Test improvements"
gh api repos/$REPO/labels -f name="type/chore" -f color="ffffff" -f description="Maintenance tasks"

# Priority labels
gh api repos/$REPO/labels -f name="priority/high" -f color="b60205" -f description="Critical path"
gh api repos/$REPO/labels -f name="priority/medium" -f color="e99695" -f description="Important but not blocking"
gh api repos/$REPO/labels -f name="priority/low" -f color="c2e0c6" -f description="Nice to have"

# Status labels
gh api repos/$REPO/labels -f name="status/ready" -f color="0e8a16" -f description="Ready to work on"
gh api repos/$REPO/labels -f name="status/in-progress" -f color="fbca04" -f description="Currently being worked on"
gh api repos/$REPO/labels -f name="status/blocked" -f color="d93f0b" -f description="Blocked by another issue"
gh api repos/$REPO/labels -f name="status/review" -f color="5319e7" -f description="In review"

# Milestone labels
gh api repos/$REPO/labels -f name="m1/foundation" -f color="bfdadc" -f description="Milestone 1: Foundation"
gh api repos/$REPO/labels -f name="m2/github-adapter" -f color="bfdadc" -f description="Milestone 2: GitHub Adapter"
gh api repos/$REPO/labels -f name="m3/soda-integration" -f color="bfdadc" -f description="Milestone 3: soda Integration"
gh api repos/$REPO/labels -f name="m4/frontend" -f color="bfdadc" -f description="Milestone 4: Frontend"
gh api repos/$REPO/labels -f name="m5/self-host" -f color="bfdadc" -f description="Milestone 5: Self-Host"

echo "GitHub structure setup complete!"
echo ""
echo "Next: Create issues for M1 (Foundation) milestone"
