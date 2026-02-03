# Documentation Deployment Guide

This guide explains how to build, preview, and deploy the AgentFense documentation.

## Local Development

### Prerequisites

```bash
# Install documentation dependencies
pip install -r requirements-docs.txt

# Or install from mkdocs.yml requirements
pip install mkdocs-material mkdocs-i18n mkdocstrings[python]
```

### Build and Preview

```bash
# Serve documentation locally (with live reload)
mkdocs serve

# Open http://localhost:8000 in your browser

# Build static site (outputs to ./site/)
mkdocs build

# Build with strict mode (fails on warnings)
mkdocs build --strict
```

### Test Different Languages

```bash
# English (default)
mkdocs serve

# Chinese
# The i18n plugin handles language switching automatically
# Navigate to /zh/ in your browser
```

## GitHub Pages Deployment

### Automatic Deployment

Documentation is automatically deployed when:

1. **Main branch**: Commits to `main` that modify docs trigger deployment
   - Workflow: `.github/workflows/docs.yml`
   - URL: https://ajaxzhan.github.io/AgentFense/

2. **Version tags**: Tags like `v1.0.0` trigger versioned deployment
   - Workflow: `.github/workflows/docs-versioned.yml`
   - Uses `mike` for version management
   - Creates version-specific URLs

### Manual Deployment

#### Deploy Latest (Main Branch)

```bash
# This happens automatically on push to main
# To trigger manually, use GitHub Actions UI:
# Actions → Documentation → Run workflow
```

#### Deploy Specific Version

```bash
# Option 1: Push a version tag
git tag v1.0.0
git push origin v1.0.0

# Option 2: Use GitHub Actions UI
# Actions → Versioned Documentation → Run workflow
# Enter version: 1.0.0
# Set as latest: true/false
```

#### Local Deployment with mike

```bash
# Install mike
pip install mike

# Deploy version and set as latest
mike deploy --push --update-aliases 1.0.0 latest

# Deploy version without setting as latest
mike deploy --push 1.0.0

# Set default version
mike set-default --push latest

# List all versions
mike list

# Delete a version
mike delete --push 1.0.0
```

## Version Management

### Version Naming

Follow semantic versioning:
- Major releases: `1.0.0`, `2.0.0`
- Minor releases: `1.1.0`, `1.2.0`
- Patch releases: `1.0.1`, `1.0.2`

Special versions:
- `latest`: Always points to the newest stable release
- `dev`: Development version (optional, for pre-release docs)

### Version Lifecycle

1. **Development**: Edit docs in `docs/` directory
2. **Preview**: Use `mkdocs serve` to preview locally
3. **Release**: Tag release (e.g., `v1.0.0`)
4. **Deploy**: GitHub Actions automatically deploys version
5. **Update**: `latest` alias updated to point to new version

### Managing Multiple Versions

```bash
# Deploy multiple versions
mike deploy --push 0.9.0
mike deploy --push 1.0.0 latest
mike deploy --push 1.1.0 latest

# Users can switch versions using the version selector
# in the documentation header
```

## Troubleshooting

### Build Failures

**Problem**: `mkdocs build` fails with import errors

**Solution**:
```bash
# Install Python SDK in development mode
pip install -e sdk/python/

# Verify imports work
python -c "import agentfense; print(agentfense.__version__)"
```

**Problem**: Broken links or missing pages

**Solution**:
```bash
# Build with strict mode to catch issues
mkdocs build --strict --verbose

# Check navigation in mkdocs.yml matches actual files
```

### GitHub Pages Issues

**Problem**: Pages not updating after deployment

**Solution**:
1. Check GitHub Actions workflow status
2. Verify Pages is enabled in repo settings
3. Ensure correct branch is selected (usually `gh-pages`)
4. Wait a few minutes for CDN cache to clear

**Problem**: 404 errors on versioned docs

**Solution**:
```bash
# Verify versions are deployed
mike list

# Check that gh-pages branch has versions/ directory
git checkout gh-pages
ls -la versions/
```

## Configuration

### mkdocs.yml

Key configuration sections:

```yaml
# Theme and appearance
theme:
  name: material
  palette: [...]

# Plugins (order matters!)
plugins:
  - search
  - i18n  # Must come before mkdocstrings
  - mkdocstrings

# Navigation structure
nav:
  - Get Started: [...]
  - SDK Documentation: [...]
```

### GitHub Actions

**docs.yml**: Main deployment workflow
- Triggers: Push to main, PR to main
- Builds and deploys to Pages
- URL: Base site URL

**docs-versioned.yml**: Version deployment workflow
- Triggers: Version tags, manual dispatch
- Uses mike for version management
- URL: Base site URL with version selector

## Best Practices

1. **Always test locally** before pushing:
   ```bash
   mkdocs build --strict
   ```

2. **Update both languages** when editing docs

3. **Keep navigation in sync** between mkdocs.yml and actual files

4. **Version docs with releases**: Tag docs at the same time as code releases

5. **Use semantic versioning**: Follow semver for version numbers

6. **Archive old versions**: Keep at least the last 3 major versions accessible

## References

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
- [mike (Versioning)](https://github.com/jimporter/mike)
- [GitHub Pages](https://docs.github.com/en/pages)
