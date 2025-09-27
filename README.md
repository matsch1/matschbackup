<div id="top">

<!-- HEADER STYLE: CLASSIC -->
<div align="center">

<img src="readmeai/assets/logos/purple.svg" width="30%" style="position: relative; top: 0; right: 0;" alt="Project Logo"/>

# MATSCHBACKUP

<em></em>

<!-- BADGES -->
<!-- local repository, no metadata badges. -->

<em>Built with the tools and technologies:</em>

<img src="https://img.shields.io/badge/Go-00ADD8.svg?style=default&logo=Go&logoColor=white" alt="Go">
<img src="https://img.shields.io/badge/Kong-003459.svg?style=default&logo=Kong&logoColor=white" alt="Kong">

</div>
<br>

---

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [Features](#features)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
    - [Usage](#usage)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

This is a personal, automated backup solution written in Go.

  ### Core Features

- Intelligent Scheduling: Only runs a backup if the last one is older than a configurable threshold (--max-days).
- Concurrency: Supports parallel backups of multiple paths to maximize efficiency.
- Retention Policy: Automatically purges the oldest backup to stay within a defined --max-backups limit.
- Compression: Zips directories (--zip) to reduce transfer size and disk space on the remote.
- Verification: Guarantees a valid backup by creating a BACKUP_COMPLETED file upon successful transfer.
- Dry Run: Use --dry-run to test your configuration without any file changes.

---

## Features

<code>❯ REPLACE-ME</code>

---

## Project Structure

```sh
└── matschbackup/
    ├── README.md
    ├── Taskfile.yml
    ├── backup
    ├── go.mod
    ├── go.sum
    ├── internal
    │   ├── remote
    │   └── utils
    ├── main.go
    └── pkg
        ├── externalCommand.go
        ├── file.go
        └── zip.go
```

---

## Getting Started

### Prerequisites

This project requires the following dependencies:

- **Programming Language:** Go
- **Package Manager:** Go modules
- **Additional software:** 
  - rclone with backup location
  - (optional) task

### Installation

Build matschbackup from the source and install dependencies:

1. **Clone the repository:**

    ```sh
    git clone ../matschbackup
    ```

2. **Navigate to the project directory:**

    ```sh
    cd matschbackup
    ```

3. **Install the dependencies:**

**Using Task:**

    ```sh
    task build
    ```
  
**Using Go:**

    ```sh
    go build -o matschbackup
    ```

### Usage

    ```sh
    ./matschbackup \
      --path /home/user/documents \
      --path /home/user/projects/my-project \
      --remote "fritznas:/backups" \
      --concurrency 5 \
      --zip \
      --max-backups 7
    ```

#### Command-Line Flags

| Flag           | Type      | Default | Description                                   |
|----------------|-----------|---------|-----------------------------------------------|
| --path         | []string  |         | Required. Local paths to back up. Can be used multiple times. |
| --remote       | string    |         | Required. The rclone target path.            |
| --max-backups  | int       | 14      | Max number of backups to keep.               |
| --max-days     | int       | 1       | Min. age in days for a new backup.          |
| --concurrency  | int       | 2       | Max. number of parallel backups.            |
| --zip, -z      | bool      | false   | Compress directories before copying.        |
| --dry-run      | bool      | false   | Simulate the process without changes.       |
| --debug, -d    | bool      | false   | Enable verbose debug output.                |

---

## Roadmap

- [X] Backup is working
- [X] Add backup internal
- [X] zip files before backup
- [X] parallel backups of directories
- [X] Add files if backup is complete
- [ ] Add useful logs when not in debugging mode
- [ ] Safe log file additionally
- [ ] Add time measurement for backup and write this in log file

---

## Contributing

- **💬 [Join the Discussions](https://LOCAL/src/matschbackup/discussions)**: Share your insights, provide feedback, or ask questions.
- **🐛 [Report Issues](https://LOCAL/src/matschbackup/issues)**: Submit bugs found or log feature requests for the `matschbackup` project.
- **💡 [Submit Pull Requests](https://LOCAL/src/matschbackup/blob/main/CONTRIBUTING.md)**: Review open PRs, and submit your own PRs.

<details closed>
<summary>Contributing Guidelines</summary>

1. **Fork the Repository**: Start by forking the project repository to your LOCAL account.
2. **Clone Locally**: Clone the forked repository to your local machine using a git client.
   ```sh
   git clone /home/matsch/src/matschbackup
   ```
3. **Create a New Branch**: Always work on a new branch, giving it a descriptive name.
   ```sh
   git checkout -b new-feature-x
   ```
4. **Make Your Changes**: Develop and test your changes locally.
5. **Commit Your Changes**: Commit with a clear message describing your updates.
   ```sh
   git commit -m 'Implemented new feature x.'
   ```
6. **Push to LOCAL**: Push the changes to your forked repository.
   ```sh
   git push origin new-feature-x
   ```
7. **Submit a Pull Request**: Create a PR against the original project repository. Clearly describe the changes and their motivations.
8. **Review**: Once your PR is reviewed and approved, it will be merged into the main branch. Congratulations on your contribution!
</details>

<details closed>
<summary>Contributor Graph</summary>
<br>
<p align="left">
   <a href="https://LOCAL{/src/matschbackup/}graphs/contributors">
      <img src="https://contrib.rocks/image?repo=src/matschbackup">
   </a>
</p>
</details>

---

## License

Matschbackup is protected under the [LICENSE](https://choosealicense.com/licenses) License. For more details, refer to the [LICENSE](https://choosealicense.com/licenses/) file.

---


<div align="right">

[![][back-to-top]](#top)

</div>


[back-to-top]: https://img.shields.io/badge/-BACK_TO_TOP-151515?style=flat-square


---
