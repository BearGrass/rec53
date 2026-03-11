# Environment Detection Rules

## Language Detection

Detect primary language from config files in project root:

| Config File | Language | Commands |
|-------------|----------|----------|
| `go.mod` | Go | See Go section below |
| `package.json` | JavaScript/TypeScript | See JS/TS section below |
| `pyproject.toml`, `requirements.txt`, `setup.py` | Python | See Python section below |
| `Cargo.toml` | Rust | See Rust section below |
| `pom.xml`, `build.gradle` | Java | See Java section below |
| `Gemfile`, `*.gemspec` | Ruby | See Ruby section below |
| `composer.json` | PHP | See PHP section below |
| `Makefile`, `CMakeLists.txt`, `meson.build`, `*.c`, `*.h` | C | See C section below |
| `Makefile`, `CMakeLists.txt`, `*.cpp`, `*.hpp` | C++ | See C++ section below |

## Language-Specific Commands

### Go

```bash
BUILD_CMD="go build ./..."
TEST_CMD="go test ./..."
LINT_CMD="go vet ./..."
TEST_COVER_CMD="go test -coverprofile=coverage.out ./..."
RACE_FLAG="-race"
```

**Additional checks:**
- Check for `Makefile` with custom targets
- Check for `golangci-lint` configuration

### JavaScript/TypeScript

```bash
BUILD_CMD="npm run build"
TEST_CMD="npm test"
LINT_CMD="npm run lint"
TEST_COVER_CMD="npx jest --coverage"
RACE_FLAG=""  # Not applicable
```

**Package manager detection:**
- `pnpm-lock.yaml` → use `pnpm` commands
- `yarn.lock` → use `yarn` commands
- `package-lock.json` → use `npm` commands

**Framework detection:**
- `vite.config.*` → Vite project
- `next.config.*` → Next.js project
- `vue.config.*` → Vue CLI project

### Python

```bash
BUILD_CMD="pip install -e ."
TEST_CMD="pytest"
LINT_CMD="ruff check ."
TEST_COVER_CMD="pytest --cov"
RACE_FLAG=""  # Not applicable
```

**Virtual environment:**
- Check for `.venv/`, `venv/`, `env/`
- Note: Commands run in activated venv

**Test framework detection:**
- `pytest.ini`, `pyproject.toml` with pytest config → pytest
- `setup.cfg` with unittest → unittest

### Rust

```bash
BUILD_CMD="cargo build"
TEST_CMD="cargo test"
LINT_CMD="cargo clippy"
TEST_COVER_CMD="cargo tarpaulin"
RACE_FLAG=""  # Handled by language
```

### Java

```bash
BUILD_CMD="mvn compile"  # or "./gradlew build"
TEST_CMD="mvn test"       # or "./gradlew test"
LINT_CMD="mvn checkstyle:check"
TEST_COVER_CMD="mvn jacoco:report"
RACE_FLAG=""  # Not applicable
```

**Build tool detection:**
- `pom.xml` → Maven
- `build.gradle` or `build.gradle.kts` → Gradle

### Ruby

```bash
BUILD_CMD="bundle install"
TEST_CMD="bundle exec rspec"  # or "bundle exec rake test"
LINT_CMD="bundle exec rubocop"
TEST_COVER_CMD="COVERAGE=true bundle exec rspec"
RACE_FLAG=""  # Not applicable
```

### PHP

```bash
BUILD_CMD="composer install"
TEST_CMD="vendor/bin/phpunit"
LINT_CMD="vendor/bin/phpcs"
TEST_COVER_CMD="vendor/bin/phpunit --coverage-html coverage"
RACE_FLAG=""  # Not applicable
```

### C

**Build system detection:**

```bash
# Makefile (most common)
BUILD_CMD="make build"
TEST_CMD="make test"
LINT_CMD="make lint"
TEST_COVER_CMD="make coverage"
RACE_FLAG=""  # Not applicable

# CMake
BUILD_CMD="cmake -B build && cmake --build build"
TEST_CMD="ctest --test-dir build"
LINT_CMD="cppcheck src/"
TEST_COVER_CMD="cmake -B build -DENABLE_COVERAGE=ON && cmake --build build && ctest --test-dir build"
RACE_FLAG=""  # Not applicable

# Meson
BUILD_CMD="meson setup build && meson compile -C build"
TEST_CMD="meson test -C build"
LINT_CMD="cppcheck src/"
TEST_COVER_CMD="meson setup build -Db_coverage=true && meson test -C build"
RACE_FLAG=""  # Not applicable
```

**Coverage detection:**
- Check for `gcov`, `lcov`, or CMake `ENABLE_COVERAGE` flag
- Common coverage setup: compile with `-fprofile-arcs -ftest-coverage`

**Standard directory structure:**
- `src/` or `lib/` - source files
- `include/` or `inc/` - header files
- `test/` or `tests/` - test files
- `examples/` or `examples/` - example usage

**Common linters:**
- `cppcheck` - static analysis
- `clang-tidy` - clang-based linter
- `splint` - secure programming linter

### C++

**Build system detection:**

```bash
# Makefile
BUILD_CMD="make build"
TEST_CMD="make test"
LINT_CMD="make lint"
TEST_COVER_CMD="make coverage"
RACE_FLAG=""  # Not applicable

# CMake
BUILD_CMD="cmake -B build && cmake --build build"
TEST_CMD="ctest --test-dir build"
LINT_CMD="clang-tidy src/**/*.cpp"
TEST_COVER_CMD="cmake -B build -DENABLE_COVERAGE=ON && cmake --build build && ctest --test-dir build"
RACE_FLAG=""  # Not applicable

# Meson
BUILD_CMD="meson setup build && meson compile -C build"
TEST_CMD="meson test -C build"
LINT_CMD="clang-tidy src/**/*.cpp"
TEST_COVER_CMD="meson setup build -Db_coverage=true && meson test -C build"
RACE_FLAG=""  # Not applicable
```

**Test framework detection:**
- `Unity` - embedded testing framework
- `CMocka` - mock object library
- `Check` - unit testing framework
- `Google Test` (C++) - `gtest`
- `Catch2` (C++) - header-only testing

### Other Languages

If language cannot be auto-detected:

1. **Ask user for commands:**
   ```
   I couldn't auto-detect your project's language.
   Please provide:
   - Build command:
   - Test command:
   - Lint command (if any):
   - Test coverage command (if any):
   ```

2. **Set reasonable defaults:**
   - `BUILD_CMD="make build"` (if Makefile exists)
   - `TEST_CMD="make test"` (if Makefile exists)

## Existing Files Check

### CLAUDE.md

If exists:
```
⚠️ CLAUDE.md already exists.

Current sections: [list sections]

Options:
1. Augment existing CLAUDE.md (recommended)
2. Create backup and overwrite
3. Cancel setup

How would you like to proceed?
```

### .claude/skills/

If exists:
```
⚠️ Project skills directory already exists with skills:
- [list existing skills]

Setup will create:
- plan/SKILL.md
- dev/SKILL.md
- dev-resume/SKILL.md
- test/SKILL.md
- test-resume/SKILL.md
- sync-docs/SKILL.md

Continue? This may overwrite existing skills with same names.
```

### DOC_DIR

If exists:
```
⚠️ Documentation directory {DOC_DIR} already exists with files:
- [list files]

Setup will AUGMENT existing files, not overwrite.
Missing files will be created.
Continue?
```

## Variable Summary

After detection, set these variables for template substitution:

| Variable | Description | Example |
|----------|-------------|---------|
| `LANG` | Primary language | `go`, `typescript`, `python`, `c`, `cpp` |
| `BUILD_CMD` | Build command | `go build ./...`, `cmake --build build` |
| `TEST_CMD` | Test command | `go test ./...`, `ctest --test-dir build` |
| `LINT_CMD` | Lint command | `go vet ./...`, `cppcheck src/` |
| `TEST_COVER_CMD` | Coverage command | `go test -coverprofile=coverage.out ./...`, `gcov *.c` |
| `RACE_FLAG` | Race detection flag | `-race` or empty |
| `DOC_DIR` | Documentation directory | `.rec53`, `docs` |