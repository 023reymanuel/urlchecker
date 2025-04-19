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

## Usage

Run `urlcheck --help` to see available commands and options.

**Commands**:
- `urlcheck check <url>`: Check a single URL for status and response time.
  - `--keyword`, `-k`: Search for a keyword in the page content (case-insensitive).
- `urlcheck list <file>`: Check multiple URLs from a file (one URL per line).
  - `--keyword`, `-k`: Search for a keyword in all pages.
  - `--output`, `-o`: Save results to a file (CSV or JSON based on --format).

**Syntax**:
```bash
urlcheck [command] [arguments] [flags]