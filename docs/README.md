# AgentFense Documentation

This directory contains the complete documentation for AgentFense, built with MkDocs Material and deployed to GitHub Pages.

## 📚 Documentation Structure

```
docs/
├── en/                          # English documentation
│   ├── index.md                 # Homepage
│   ├── get-started/             # Quick start guides
│   │   ├── quickstart.md
│   │   ├── concepts.md
│   │   └── installation.md
│   ├── why/                     # Value proposition
│   │   ├── index.md
│   │   ├── use-cases.md
│   │   ├── comparison.md
│   │   └── architecture.md
│   ├── security/                # Security model
│   │   ├── index.md
│   │   ├── permissions.md
│   │   ├── presets.md
│   │   └── best-practices.md
│   ├── sdk/                     # SDK documentation
│   │   ├── index.md
│   │   └── python/              # Python SDK
│   │       ├── overview.md
│   │       ├── high-level.md
│   │       ├── async.md
│   │       ├── sessions.md
│   │       ├── permissions.md
│   │       └── errors.md
│   ├── reference/               # API reference
│   │   ├── index.md
│   │   ├── python.md            # Auto-generated
│   │   ├── go.md
│   │   └── grpc.md
│   └── faq/                     # FAQ
│       ├── index.md
│       ├── common-issues.md
│       └── performance.md
├── zh/                          # Chinese documentation (mirrors en/)
├── stylesheets/
│   └── extra.css                # Custom styles
├── DEPLOYMENT.md                # Deployment guide
└── README.md                    # This file
```

## 🌍 Languages

Documentation is available in:
- **English** (default): `/en/` or root `/`
- **中文**: `/zh/`

Both versions are maintained in parallel with the same structure.

## 🚀 Quick Start

### View Documentation Locally

```bash
# Install dependencies
pip install -r requirements-docs.txt

# Serve with live reload
mkdocs serve

# Open http://localhost:8000
```

### Build Static Site

```bash
# Build to ./site/
mkdocs build

# Build with strict mode (recommended)
mkdocs build --strict
```

## 📝 Writing Documentation

### Adding a New Page

1. Create the Markdown file in both `en/` and `zh/` directories:
   ```bash
   docs/en/new-section/new-page.md
   docs/zh/new-section/new-page.md
   ```

2. Add to navigation in `mkdocs.yml`:
   ```yaml
   nav:
     - New Section:
       - New Page: new-section/new-page.md
   ```

3. Update the i18n translations in `mkdocs.yml`:
   ```yaml
   plugins:
     - i18n:
         languages:
           - locale: zh
             nav_translations:
               New Section: 新章节
               New Page: 新页面
   ```

### Writing Style

Follow these guidelines:
- **Problem-first**: Start with user pain points
- **Value-driven**: Explain why, not just how
- **Code examples**: Include complete, runnable examples
- **Bilingual**: Maintain both English and Chinese versions
- **Consistent terminology**: Use the same terms throughout

### Markdown Extensions

We support:
- **Admonitions**: `!!! note`, `!!! warning`, etc.
- **Code blocks**: With syntax highlighting and copy button
- **Tabs**: For multi-language examples
- **Tables**: Standard GitHub Flavored Markdown
- **Mermaid**: Diagrams (use `mermaid` code blocks)

Example:
```markdown
!!! tip "Pro Tip"
    Use the `agent-safe` preset for most AI agent scenarios.

\`\`\`python
from agentfense import Sandbox

with Sandbox.from_local("./project") as sandbox:
    result = sandbox.run("python main.py")
\`\`\`
```

## 🔄 Deployment

### Automatic Deployment

- **Main branch**: Every push to `main` triggers deployment to https://ajaxzhan.github.io/AgentFense/
- **Version tags**: Tags like `v1.0.0` create versioned documentation

### Manual Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed instructions.

## 📖 API Reference

Python API reference is auto-generated from source code using `mkdocstrings`.

To document a function:
```python
def example_function(param: str) -> bool:
    """Short description.

    Longer description with details.

    Args:
        param: Parameter description.

    Returns:
        Return value description.

    Raises:
        ValueError: When parameter is invalid.

    Example:
        ```python
        result = example_function("test")
        ```
    """
    return True
```

## 🎨 Customization

### Theme Colors

Edit in `mkdocs.yml`:
```yaml
theme:
  palette:
    - scheme: default
      primary: indigo
      accent: indigo
```

### Custom CSS

Add styles to `docs/stylesheets/extra.css`.

### Logo and Favicon

Place files in `docs/assets/` and reference in `mkdocs.yml`:
```yaml
theme:
  logo: assets/logo.png
  favicon: assets/favicon.ico
```

## 🔍 Search

Search is powered by the built-in MkDocs search plugin with:
- Multi-language support
- Search suggestions
- Keyboard shortcuts (press `/` to focus search)

## 📊 Analytics

Google Analytics can be enabled in `mkdocs.yml`:
```yaml
extra:
  analytics:
    provider: google
    property: G-XXXXXXXXXX
```

## 🐛 Troubleshooting

### Build Errors

**Problem**: Import errors when building
```
ModuleNotFoundError: No module named 'agentfense'
```

**Solution**:
```bash
pip install -e sdk/python/
```

**Problem**: Broken links
```
WARNING - Doc file 'path/to/file.md' contains a link to 'broken-link.md', but this file does not exist.
```

**Solution**:
- Check the link path is correct
- Ensure the file exists in both `en/` and `zh/`
- Use relative paths from the current file

### Deployment Issues

See [DEPLOYMENT.md](DEPLOYMENT.md) for deployment troubleshooting.

## 📚 References

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
- [mkdocstrings](https://mkdocstrings.github.io/)
- [mike (Versioning)](https://github.com/jimporter/mike)

## 🤝 Contributing

To contribute to documentation:

1. Fork the repository
2. Create a branch for your changes
3. Edit documentation in both `en/` and `zh/` directories
4. Test locally with `mkdocs serve`
5. Submit a pull request

Documentation PRs are automatically tested by GitHub Actions.

## 📜 License

Documentation is licensed under the same license as AgentFense (MIT).
