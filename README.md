# urlcheck

A  CLI tool to check the status, response time, and content of URLs. Useful for developers, site owners, and sysadmins to monitor website availability and content integrity.

## Features
- Check a single URL (`urlcheck check <url>`).
- Check multiple URLs from a file (`urlcheck list <file>`).
- Search for keywords on pages (`--keyword`).
- Export results to CSV (`--output`).
- Concurrent URL checking for efficiency.

## Installation
```bash
go install github.com/<your-username>/urlcheck@latest

# Check a single URL
urlcheck check https://example.com

# Check with a keyword
urlcheck check https://example.com --keyword "Example Domain"

# Check URLs from a file and save to CSV
urlcheck list urls.txt --output results.csv