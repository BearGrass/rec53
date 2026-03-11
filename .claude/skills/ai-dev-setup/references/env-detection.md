# Environment Detection Rules

## Language Detection

| Config File | Language |
|-------------|----------|
| `go.mod` | Go |
| `package.json` | JavaScript/TypeScript |
| `pyproject.toml`, `requirements.txt`, `setup.py` | Python |
| `Cargo.toml` | Rust |
| `pom.xml`, `build.gradle` | Java |
| `Gemfile`, `*.gemspec` | Ruby |
| `composer.json` | PHP |
| `Makefile`, `CMakeLists.txt`, `*.c`, `*.h` | C |
| `Makefile`, `CMakeLists.txt`, `*.cpp`, `*.hpp` | C++ |

## Commands by Language

| Language | BUILD_CMD | TEST_CMD | LINT_CMD | TEST_COVER_CMD | RACE_FLAG |
|----------|-----------|----------|----------|----------------|-----------|
| Go | `go build ./...` | `go test ./...` | `go vet ./...` | `go test -coverprofile=coverage.out ./...` | `-race` |
| JS/TS (npm) | `npm run build` | `npm test` | `npm run lint` | `npx jest --coverage` | |
| Python | `pip install -e .` | `pytest` | `ruff check .` | `pytest --cov` | |
| Rust | `cargo build` | `cargo test` | `cargo clippy` | `cargo tarpaulin` | |
| Java (Maven) | `mvn compile` | `mvn test` | `mvn checkstyle:check` | `mvn jacoco:report` | |
| Java (Gradle) | `./gradlew build` | `./gradlew test` | `./gradlew check` | `./gradlew jacocoTestReport` | |
| Ruby | `bundle install` | `bundle exec rspec` | `bundle exec rubocop` | `COVERAGE=true bundle exec rspec` | |
| PHP | `composer install` | `vendor/bin/phpunit` | `vendor/bin/phpcs` | `vendor/bin/phpunit --coverage-html coverage` | |
| C/C++ (Make) | `make build` | `make test` | `make lint` | `make coverage` | |
| C/C++ (CMake) | `cmake -B build && cmake --build build` | `ctest --test-dir build` | `cppcheck src/` | `cmake -B build -DENABLE_COVERAGE=ON && cmake --build build && ctest --test-dir build` | |

## Sub-Detection Rules

**JS/TS package manager:** `pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, otherwise npm

**JS/TS framework:** `vite.config.*` → Vite, `next.config.*` → Next.js, `vue.config.*` → Vue CLI

**Java build tool:** `pom.xml` → Maven, `build.gradle` → Gradle

**Go extras:** check Makefile for custom targets, check for golangci-lint config

**Python venv:** check for `.venv/`, `venv/`, `env/` — note commands run in activated venv

**Unknown language:** ask user for BUILD_CMD, TEST_CMD, LINT_CMD, TEST_COVER_CMD

## Variable Summary

| Variable | Description |
|----------|-------------|
| `LANG` | Primary language: `go`, `typescript`, `python`, `rust`, `java`, `ruby`, `php`, `c`, `cpp` |
| `BUILD_CMD` | Build command |
| `TEST_CMD` | Test command |
| `LINT_CMD` | Lint command |
| `TEST_COVER_CMD` | Coverage command |
| `RACE_FLAG` | `-race` (Go only) or empty |
| `DOC_DIR` | Documentation directory (default: `.docs`, or $ARGUMENTS if provided) |

## Existing Files — Conflict Handling

**CLAUDE.md exists:**
Show current sections. Offer:
1. Augment existing (recommended)
2. Backup and overwrite
3. Cancel

**`.claude/skills/` exists:**
List existing skills. Warn about overwrites for: plan, dev, dev-resume, test, test-resume, sync-docs. Ask to continue.

**DOC_DIR exists:**
List existing files. Augment (don't overwrite), create missing files. Ask to continue.
