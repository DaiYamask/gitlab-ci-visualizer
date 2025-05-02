# GitLab CI Visualizer (gitlabviz)

A command-line tool to visualize GitLab CI pipelines, with a focus on showing which jobs will be executed based on different rule conditions.

## Features

- Parse `.gitlab-ci.yml` files
- Visualize pipeline structure with stages and jobs
- Group jobs by rule conditions to understand what will run under different scenarios
- Filter jobs by stage or rule condition
- Display workflow rules
- Color-coded output for better readability
- Multiple output formats (text, JSON, YAML, table)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/gitlab-ci-visualizer.git
cd gitlab-ci-visualizer

# Build using make
make build

# Or build manually
go build -o bin/gitlabviz ./cmd/gitlabviz

# Optional: Install to a directory in your PATH
sudo make install
# Or manually
sudo cp bin/gitlabviz /usr/local/bin/
```

### Download Binary

You can also download pre-built binaries from the [releases page](https://github.com/yourusername/gitlab-ci-visualizer/releases).

## Usage

```bash
# Show visualization of a GitLab CI pipeline
gitlabviz show path/to/.gitlab-ci.yml

# Filter jobs by stage
gitlabviz show --stage test path/to/.gitlab-ci.yml

# Filter jobs by rule condition
gitlabviz show --rule '$CI_COMMIT_BRANCH == "main"' path/to/.gitlab-ci.yml

# Combine filters
gitlabviz show --stage test --rule '$CI_COMMIT_TAG' path/to/.gitlab-ci.yml

# Disable colored output
gitlabviz show --no-color path/to/.gitlab-ci.yml

# Output in different formats
gitlabviz show --format json path/to/.gitlab-ci.yml
gitlabviz show --format yaml path/to/.gitlab-ci.yml
gitlabviz show --format table path/to/.gitlab-ci.yml

# Get help
gitlabviz --help
gitlabviz show --help
```

## Example Output

### Basic Visualization

```
GitLab CI Pipeline Visualization
===============================

Stages:
  1. build
  2. test
  3. deploy

Jobs by Rules:

  Rule Condition: $CI_COMMIT_BRANCH == "main"
    - build (build)
      Rule 1: if: $CI_COMMIT_BRANCH == "main", when: always
      Rule 2: if: $CI_PIPELINE_SOURCE == "merge_request_event", when: always
    - test:unit (test)
      Rule 1: if: $CI_COMMIT_BRANCH == "main", when: always
      Rule 2: if: $CI_PIPELINE_SOURCE == "merge_request_event", when: always
    - test:integration (test)
      Rule 1: if: $CI_COMMIT_BRANCH == "main", when: always
      Rule 2: if: $CI_PIPELINE_SOURCE == "merge_request_event" && $CI_MERGE_REQUEST_LABELS =~ /integration-tests/, when: always
      Rule 3: when: never
    - deploy:staging (deploy)
      Rule 1: if: $CI_COMMIT_BRANCH == "main", when: manual
      Rule 2: when: never
```

### With Workflow Rules

```
GitLab CI Pipeline Visualization
===============================

Stages:
  1. prepare
  2. build
  3. test
  4. deploy
  5. cleanup

Workflow Rules:
  1. if: $CI_PIPELINE_SOURCE == "merge_request_event"
  2. if: $CI_COMMIT_BRANCH == "main"
  3. if: $CI_COMMIT_TAG

Jobs by Rules:
  ...
```

### With Filters Applied

```
GitLab CI Pipeline Visualization
===============================

Stages:
  1. prepare
  2. build
  3. test
  4. deploy
  5. cleanup

Jobs by Rules:

  Rule Condition: $CI_COMMIT_TAG
    - test:unit (test)
      Rule 1: if: $CI_COMMIT_BRANCH == "main", when: always
      Rule 2: if: $CI_PIPELINE_SOURCE == "merge_request_event", when: always
      Rule 3: if: $CI_COMMIT_TAG, when: always

Filters applied:
  Stage: test
  Rule: $CI_COMMIT_TAG
```

## Development

### Project Structure

```
gitlab-ci-visualizer/
├── cmd/                    # Application entry points
│   └── gitlabviz/          # Main application command
├── internal/               # Private application code
│   ├── parser/             # YAML parsing components
│   └── visualizer/         # Visualization components
├── examples/               # Example .gitlab-ci.yml files
├── bin/                    # Build output directory
├── Makefile                # Build automation
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── README.md               # Project documentation
```

### Building for Different Platforms

```bash
# Build for all platforms
make build-all

# Build for specific platform
GOOS=darwin GOARCH=amd64 go build -o bin/gitlabviz-darwin-amd64 ./cmd/gitlabviz
```

## License

MIT
