#!/bin/bash
# Verify documentation system completeness

set -e

echo "════════════════════════════════════════════════════════════════"
echo "  Verifying AgentFense Documentation System"
echo "════════════════════════════════════════════════════════════════"
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} $1"
        return 0
    else
        echo -e "${RED}✗${NC} $1 (missing)"
        return 1
    fi
}

check_dir() {
    if [ -d "$1" ]; then
        echo -e "${GREEN}✓${NC} $1/"
        return 0
    else
        echo -e "${RED}✗${NC} $1/ (missing)"
        return 1
    fi
}

ERRORS=0

echo "📁 Core Configuration Files:"
check_file "mkdocs.yml" || ((ERRORS++))
check_file "requirements-docs.txt" || ((ERRORS++))
check_file "docs/README.md" || ((ERRORS++))
check_file "docs/DEPLOYMENT.md" || ((ERRORS++))
check_file "DOCS_SUMMARY.md" || ((ERRORS++))
echo

echo "🎨 Styling:"
check_file "docs/stylesheets/extra.css" || ((ERRORS++))
echo

echo "⚙️  GitHub Actions:"
check_file ".github/workflows/docs.yml" || ((ERRORS++))
check_file ".github/workflows/docs-versioned.yml" || ((ERRORS++))
echo

echo "📚 English Documentation:"
check_file "docs/en/index.md" || ((ERRORS++))
check_dir "docs/en/get-started" || ((ERRORS++))
check_dir "docs/en/why" || ((ERRORS++))
check_dir "docs/en/security" || ((ERRORS++))
check_dir "docs/en/sdk/python" || ((ERRORS++))
check_dir "docs/en/reference" || ((ERRORS++))
check_dir "docs/en/faq" || ((ERRORS++))
echo

echo "🇨🇳 Chinese Documentation:"
check_file "docs/zh/index.md" || ((ERRORS++))
check_dir "docs/zh/get-started" || ((ERRORS++))
check_dir "docs/zh/why" || ((ERRORS++))
check_dir "docs/zh/security" || ((ERRORS++))
check_dir "docs/zh/sdk/python" || ((ERRORS++))
check_dir "docs/zh/reference" || ((ERRORS++))
check_dir "docs/zh/faq" || ((ERRORS++))
echo

echo "📊 Statistics:"
TOTAL_MD=$(find docs -name "*.md" | wc -l)
EN_MD=$(find docs/en -name "*.md" 2>/dev/null | wc -l)
ZH_MD=$(find docs/zh -name "*.md" 2>/dev/null | wc -l)

echo "   Total Markdown files: $TOTAL_MD"
echo "   English pages: $EN_MD"
echo "   Chinese pages: $ZH_MD"
echo

echo "🔍 Testing MkDocs Configuration:"
if command -v mkdocs &> /dev/null; then
    if mkdocs build --strict --site-dir /tmp/mkdocs-test 2>&1 | tail -5; then
        echo -e "${GREEN}✓${NC} MkDocs build successful"
        rm -rf /tmp/mkdocs-test
    else
        echo -e "${RED}✗${NC} MkDocs build failed"
        ((ERRORS++))
    fi
else
    echo -e "${YELLOW}⚠${NC}  mkdocs not installed (run: pip install -r requirements-docs.txt)"
fi
echo

echo "════════════════════════════════════════════════════════════════"
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✅ All checks passed! Documentation system is complete.${NC}"
    echo
    echo "Next steps:"
    echo "  1. Enable GitHub Pages in repository settings"
    echo "  2. Push to main branch: git push origin main"
    echo "  3. Visit: https://ajaxzhan.github.io/AgentFense/"
    echo
    echo "For local preview:"
    echo "  mkdocs serve"
    echo "  Open: http://localhost:8000"
else
    echo -e "${RED}❌ Found $ERRORS error(s). Please check the missing files.${NC}"
    exit 1
fi
echo "════════════════════════════════════════════════════════════════"
