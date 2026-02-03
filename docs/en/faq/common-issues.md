# Common Issues

Solutions to frequently encountered problems.

## Installation Issues

### FUSE not installed

**Error:** `fusermount: command not found`

**Solution:**
```bash
# Ubuntu/Debian
sudo apt-get install fuse libfuse-dev

# CentOS/RHEL
sudo yum install fuse fuse-libs

# macOS
brew install macfuse
```

### Permission denied when mounting

**Error:** `fuse: failed to open /dev/fuse: Permission denied`

**Solution:**
```bash
# Add user to fuse group
sudo usermod -a -G fuse $USER

# Reload groups (or logout/login)
newgrp fuse
```

## Runtime Issues

### macOS Docker Desktop: view permission not working

**Issue:** Files with `view` permission appear as "No such file" inside containers on macOS.

**Cause:** VirtioFS limitations in Docker Desktop for Mac.

**Workaround:**
- Use `read` permission instead of `view`
- Use bwrap runtime (requires Linux)
- Use native Linux or Linux VM

### Sandbox stuck in PENDING status

**Symptoms:** `sandbox.start()` hangs or times out

**Possible causes:**
1. FUSE mount timeout
2. Docker image pull in progress
3. Resource exhaustion

**Solutions:**
```python
# Check logs
import logging
logging.basicConfig(level=logging.DEBUG)

# Increase timeout (default 30s)
sandbox = Sandbox.from_local(
    "./project",
    start_timeout=60  # 60 seconds
)

# Check server logs
docker logs agentfense-server
```

### Permission denied errors

**Error:** `Permission denied` when accessing files

**Diagnosis:**
```python
# List what's visible
files = sandbox.list_files(recursive=True)
print(files)

# Check effective permissions
# Files with 'none' won't appear at all
# Files with 'view' will appear but can't be read
```

**Solutions:**
1. Verify permission rules match your intent
2. Check pattern specificity (more specific wins)
3. Use `full-access` preset to test if it's a permission issue

## Performance Issues

### High memory usage

**Symptoms:** Memory grows over time

**Causes:**
- Too many concurrent sandboxes
- Large files in Delta Layer
- FUSE cache buildup

**Solutions:**
```python
# Limit concurrent sandboxes
from sandbox_rls import ResourceLimits

sandbox = Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=256 * 1024 * 1024  # 256 MB
    )
)

# Clean up properly
sandbox.destroy(delete_codebase=True)
```

### Slow file operations

**Symptoms:** File reads/writes are slow

**Causes:**
- Large Delta Layer
- Network latency (if server is remote)
- FUSE overhead

**Solutions:**
1. Export snapshot periodically (reduces delta)
2. Use local server when possible
3. Batch file operations
4. Use streaming for large files

## Getting Help

If you encounter an issue not listed here:

1. Check [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues)
2. Enable debug logging
3. Collect server logs
4. Create a minimal reproduction example

**Debug logging:**
```python
import logging
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
```
